// Package clusterserver provides cluster communication for TokMesh.
//
// This package handles inter-node communication in distributed deployments:
//
//   - Node discovery and membership
//   - State synchronization via Raft consensus
//   - Gossip protocol for health monitoring
//   - Data replication and consistency
//
// Communication uses Connect RPC with Protobuf serialization over mTLS.
//
// @req RQ-0401
// @design DS-0401
package clusterserver
