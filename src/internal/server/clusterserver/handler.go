// Package clusterserver provides RPC handlers for cluster communication.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/yndnr/tokmesh-go/api/proto/v1"
	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// Handler implements the ClusterService RPC handlers.
//
// This connects the Connect/Protobuf RPC layer with the cluster server logic.
type Handler struct {
	server *Server
	logger *slog.Logger
}

// NewHandler creates a new RPC handler.
func NewHandler(server *Server, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		server: server,
		logger: logger,
	}
}

// Join handles the Join RPC.
//
// Allows a new node to join the cluster.
func (h *Handler) Join(
	ctx context.Context,
	req *connect.Request[v1.JoinRequest],
) (*connect.Response[v1.JoinResponse], error) {
	h.logger.Info("join request received",
		"node_id", req.Msg.NodeId,
		"addr", req.Msg.AdvertiseAddress)

	// Only the leader can accept new members
	if !h.server.IsLeader() {
		leaderID, leaderAddr := h.server.Leader()
		h.logger.Warn("join request rejected - not leader",
			"requester", req.Msg.NodeId,
			"leader_id", leaderID,
			"leader_addr", leaderAddr)

		return connect.NewResponse(&v1.JoinResponse{
			Accepted:     false,
			LeaderNodeId: leaderID,
			LeaderAddr:   leaderAddr,
		}), nil
	}

	// Apply member join through Raft
	if err := h.server.ApplyMemberJoin(req.Msg.NodeId, req.Msg.AdvertiseAddress); err != nil {
		h.logger.Error("failed to apply member join",
			"node_id", req.Msg.NodeId,
			"error", err)
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("apply member join: %w", err))
	}

	// Add to Raft cluster as voter
	if err := h.server.raft.AddVoter(req.Msg.NodeId, req.Msg.AdvertiseAddress, h.server.config.Timeouts.RaftMembership); err != nil {
		h.logger.Error("failed to add voter",
			"node_id", req.Msg.NodeId,
			"error", err)
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("add voter: %w", err))
	}

	// Prepare response with current cluster state
	members := h.server.GetMembers()
	shardMap := h.server.GetShardMap()

	pbMembers := make([]*v1.Member, 0, len(members))
	for _, m := range members {
		pbMembers = append(pbMembers, &v1.Member{
			NodeId:   m.NodeID,
			Addr:     m.Addr,
			State:    m.State,
			IsLeader: m.IsLeader,
		})
	}

	pbShards := make(map[uint32]string)
	for shardID, nodeID := range shardMap.Shards {
		pbShards[shardID] = nodeID
	}

	pbReplicas := make(map[uint32]*v1.ReplicaSet)
	for shardID, replicas := range shardMap.Replicas {
		pbReplicas[shardID] = &v1.ReplicaSet{
			NodeIds: replicas,
		}
	}

	leaderID, leaderAddr := h.server.Leader()

	h.logger.Info("join request accepted",
		"node_id", req.Msg.NodeId,
		"member_count", len(members),
		"shard_count", len(pbShards))

	return connect.NewResponse(&v1.JoinResponse{
		Accepted:     true,
		LeaderNodeId: leaderID,
		LeaderAddr:   leaderAddr,
		Members:      pbMembers,
		ShardMap: &v1.ShardMap{
			Shards:   pbShards,
			Replicas: pbReplicas,
			Version:  shardMap.Version,
		},
	}), nil
}

// GetShardMap handles the GetShardMap RPC.
//
// Returns the current shard map snapshot.
func (h *Handler) GetShardMap(
	ctx context.Context,
	req *connect.Request[v1.GetShardMapRequest],
) (*connect.Response[v1.GetShardMapResponse], error) {
	h.logger.Debug("get shard map request received")

	shardMap := h.server.GetShardMap()

	pbShards := make(map[uint32]string)
	for shardID, nodeID := range shardMap.Shards {
		pbShards[shardID] = nodeID
	}

	pbReplicas := make(map[uint32]*v1.ReplicaSet)
	for shardID, replicas := range shardMap.Replicas {
		pbReplicas[shardID] = &v1.ReplicaSet{
			NodeIds: replicas,
		}
	}

	return connect.NewResponse(&v1.GetShardMapResponse{
		ShardMap: &v1.ShardMap{
			Shards:   pbShards,
			Replicas: pbReplicas,
			Version:  shardMap.Version,
		},
	}), nil
}

// TransferShard handles the TransferShard RPC (client stream).
//
// Receives shard migration data during rebalancing.
func (h *Handler) TransferShard(
	ctx context.Context,
	stream *connect.ClientStream[v1.TransferShardRequest],
) (*connect.Response[v1.TransferShardResponse], error) {
	// Pre-flight check: Storage must be available for data migration
	// @req RQ-0401 § 1.3.1 - Storage is required for rebalancing
	if h.server.storage == nil {
		h.logger.Error("transfer shard rejected - storage engine not configured")
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("storage engine not available - cannot accept shard migration"))
	}

	h.logger.Info("transfer shard stream started")

	var (
		receivedCount uint64
		receivedBytes int64
		shardID       uint32
	)

	// Receive streaming sessions
	for stream.Receive() {
		msg := stream.Msg()

		// Record shard ID from first message
		if receivedCount == 0 {
			shardID = msg.ShardId
			h.logger.Info("shard transfer started",
				"shard_id", shardID)
		}

		// Deserialize session data
		var session domain.Session
		if err := json.Unmarshal(msg.SessionData, &session); err != nil {
			h.logger.Error("failed to unmarshal session",
				"session_id", msg.SessionId,
				"error", err)
			// Skip invalid data but continue receiving
			receivedCount++
			continue
		}

		// Apply received session to local storage
		// Note: Storage nil check done at function start, so this cannot be nil
		if err := h.server.storage.Create(ctx, &session); err != nil {
			h.logger.Error("failed to store received session - aborting migration",
				"session_id", session.ID,
				"shard_id", shardID,
				"error", err)
			// Storage failure is fatal for migration - return error
			return nil, connect.NewError(connect.CodeInternal,
				fmt.Errorf("storage failed for session %s: %w", session.ID, err))
		}

		receivedCount++
		receivedBytes += int64(len(msg.SessionData))

		h.logger.Debug("received and stored session",
			"shard_id", msg.ShardId,
			"session_id", msg.SessionId,
			"data_bytes", len(msg.SessionData))
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		if err == io.EOF {
			h.logger.Info("shard transfer completed",
				"shard_id", shardID,
				"sessions", receivedCount,
				"bytes", receivedBytes)

			return connect.NewResponse(&v1.TransferShardResponse{
				Success:          true,
				TransferredItems: receivedCount,
			}), nil
		}

		h.logger.Error("shard transfer stream error",
			"shard_id", shardID,
			"error", err)
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("stream error: %w", err))
	}

	// Transfer completed successfully
	h.logger.Info("shard transfer completed",
		"shard_id", shardID,
		"transferred_items", receivedCount)

	return connect.NewResponse(&v1.TransferShardResponse{
		Success:          true,
		TransferredItems: receivedCount,
	}), nil
}

// Ping handles the Ping RPC.
//
// Health check for cluster nodes.
func (h *Handler) Ping(
	ctx context.Context,
	req *connect.Request[v1.PingRequest],
) (*connect.Response[v1.PingResponse], error) {
	h.logger.Debug("ping received", "from", req.Msg.NodeId)

	stats := h.server.GetStats()

	return connect.NewResponse(&v1.PingResponse{
		NodeId:    stats.NodeID,
		Timestamp: time.Now().Unix(),
		IsLeader:  stats.IsLeader,
	}), nil
}
