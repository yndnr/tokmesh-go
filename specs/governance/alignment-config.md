# 配置项对齐矩阵（规范 / 需求 / 设计 / 任务）

**状态**: 草稿
**最后更新**: 2025-12-18

单一事实来源：
- 配置键字典：`specs/1-requirements/RQ-0502-配置管理需求.md`
- 配置实现设计：`specs/2-designs/DS-0502-配置管理设计.md`

| 配置键 | RQ定义 | DS引用 | TK显式引用(首处) |
|---|---|---|---|
| `security.auth.argon2.memory` | specs/1-requirements/RQ-0502-配置管理需求.md:97 | specs/2-designs/DS-0502-配置管理设计.md:80 | specs/3-tasks/TK-0201-实现安全与鉴权.md:194 |
| `server.https.enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:191 | specs/2-designs/DS-0502-配置管理设计.md:330 | specs/3-tasks/TK-0301-实现HTTP接口.md:29 |
| `server.http.address` | specs/1-requirements/RQ-0502-配置管理需求.md:204 | specs/2-designs/DS-0502-配置管理设计.md:313 | specs/3-tasks/TK-0301-实现HTTP接口.md:28 |
| `server.https.tls.cert_file` | specs/1-requirements/RQ-0502-配置管理需求.md:212 | specs/2-designs/DS-0502-配置管理设计.md:762 | specs/3-tasks/TK-0301-实现HTTP接口.md:30 |
| `server.https.tls.key_file` | specs/1-requirements/RQ-0502-配置管理需求.md:212 | specs/2-designs/DS-0502-配置管理设计.md:770 | specs/3-tasks/TK-0301-实现HTTP接口.md:30 |
| `server.https.address` | specs/1-requirements/RQ-0502-配置管理需求.md:225 | specs/2-designs/DS-0502-配置管理设计.md:328 | specs/3-tasks/TK-0301-实现HTTP接口.md:29 |
| `cluster.listen_address` | specs/1-requirements/RQ-0502-配置管理需求.md:226 | specs/2-designs/DS-0502-配置管理设计.md:350 | specs/3-tasks/TK-0401-实现分布式集群.md:161 |
| `server.redis.address` | specs/1-requirements/RQ-0502-配置管理需求.md:227 | specs/2-designs/DS-0502-配置管理设计.md:635 | specs/3-tasks/TK-0302-实现Redis协议.md:47 |
| `server.redis_tls.address` | specs/1-requirements/RQ-0502-配置管理需求.md:228 | specs/2-designs/DS-0502-配置管理设计.md:641 | specs/3-tasks/TK-0302-实现Redis协议.md:49 |
| `server.https.tls.client_ca_file` | specs/1-requirements/RQ-0502-配置管理需求.md:245 | specs/2-designs/DS-0502-配置管理设计.md:786 | specs/3-tasks/TK-0301-实现HTTP接口.md:30 |
| `cluster.tls.cert_file` | specs/1-requirements/RQ-0502-配置管理需求.md:246 | specs/2-designs/DS-0502-配置管理设计.md:826 | specs/3-tasks/TK-0401-实现分布式集群.md:172 |
| `cluster.tls.key_file` | specs/1-requirements/RQ-0502-配置管理需求.md:247 | specs/2-designs/DS-0502-配置管理设计.md:834 | specs/3-tasks/TK-0401-实现分布式集群.md:173 |
| `cluster.tls.client_ca_file` | specs/1-requirements/RQ-0502-配置管理需求.md:248 | specs/2-designs/DS-0502-配置管理设计.md:842 | specs/3-tasks/TK-0401-实现分布式集群.md:174 |
| `server.redis_tls.tls.cert_file` | specs/1-requirements/RQ-0502-配置管理需求.md:249 | specs/2-designs/DS-0502-配置管理设计.md:795 | specs/3-tasks/TK-0302-实现Redis协议.md:121 |
| `server.redis_tls.tls.key_file` | specs/1-requirements/RQ-0502-配置管理需求.md:250 | specs/2-designs/DS-0502-配置管理设计.md:803 | specs/3-tasks/TK-0302-实现Redis协议.md:122 |
| `server.redis_tls.tls.client_ca_file` | specs/1-requirements/RQ-0502-配置管理需求.md:251 | specs/2-designs/DS-0502-配置管理设计.md:817 | specs/3-tasks/TK-0302-实现Redis协议.md:123 |
| `cluster.enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:278 | specs/2-designs/DS-0502-配置管理设计.md:333 | specs/3-tasks/TK-0401-实现分布式集群.md:159 |
| `cluster.discovery.seeds` | specs/1-requirements/RQ-0502-配置管理需求.md:279 | specs/2-designs/DS-0502-配置管理设计.md:904 | specs/3-tasks/TK-0401-实现分布式集群.md:64 |
| `cluster.data.replication_factor` | specs/1-requirements/RQ-0502-配置管理需求.md:280 | specs/2-designs/DS-0502-配置管理设计.md:920 | specs/3-tasks/TK-0401-实现分布式集群.md:165 |
| `cluster.bootstrap.expect_nodes` | specs/1-requirements/RQ-0502-配置管理需求.md:281 | specs/2-designs/DS-0502-配置管理设计.md:927 | specs/3-tasks/TK-0401-实现分布式集群.md:65 |
| `cluster.rebalance.max_rate` | specs/1-requirements/RQ-0502-配置管理需求.md:282 | specs/2-designs/DS-0502-配置管理设计.md:79 | specs/3-tasks/TK-0401-实现分布式集群.md:168 |
| `cluster.rebalance.min_ttl` | specs/1-requirements/RQ-0502-配置管理需求.md:283 | specs/2-designs/DS-0502-配置管理设计.md:941 | specs/3-tasks/TK-0401-实现分布式集群.md:169 |
| `cluster.shutdown.timeout` | specs/1-requirements/RQ-0502-配置管理需求.md:284 | specs/2-designs/DS-0502-配置管理设计.md:948 | specs/3-tasks/TK-0401-实现分布式集群.md:170 |
| `server.shutdown.timeout` | specs/1-requirements/RQ-0502-配置管理需求.md:285 | specs/2-designs/DS-0502-配置管理设计.md:972 | specs/3-tasks/TK-0301-实现HTTP接口.md:32 |
| `server.shutdown.grace_period` | specs/1-requirements/RQ-0502-配置管理需求.md:286 | specs/2-designs/DS-0502-配置管理设计.md:978 | specs/3-tasks/TK-0301-实现HTTP接口.md:32 |
| `cluster.advertise_address` | specs/1-requirements/RQ-0502-配置管理需求.md:287 | specs/2-designs/DS-0502-配置管理设计.md:651 | specs/3-tasks/TK-0401-实现分布式集群.md:162 |
| `storage.wal.dir` | specs/1-requirements/RQ-0502-配置管理需求.md:295 | specs/2-designs/DS-0502-配置管理设计.md:344 | specs/3-tasks/TK-0102-实现存储引擎.md:238 |
| `storage.wal.sync_mode` | specs/1-requirements/RQ-0502-配置管理需求.md:296 | specs/2-designs/DS-0502-配置管理设计.md:999 | specs/3-tasks/TK-0102-实现存储引擎.md:239 |
| `storage.wal.sync_interval` | specs/1-requirements/RQ-0502-配置管理需求.md:297 | specs/2-designs/DS-0502-配置管理设计.md:1007 | specs/3-tasks/TK-0102-实现存储引擎.md:240 |
| `storage.wal.batch_count` | specs/1-requirements/RQ-0502-配置管理需求.md:298 | specs/2-designs/DS-0502-配置管理设计.md:1051 | specs/3-tasks/TK-0102-实现存储引擎.md:241 |
| `storage.wal.batch_size` | specs/1-requirements/RQ-0502-配置管理需求.md:299 | specs/2-designs/DS-0502-配置管理设计.md:1040 | specs/3-tasks/TK-0102-实现存储引擎.md:242 |
| `storage.snapshot.dir` | specs/1-requirements/RQ-0502-配置管理需求.md:300 | specs/2-designs/DS-0502-配置管理设计.md:345 | specs/3-tasks/TK-0102-实现存储引擎.md:243 |
| `storage.snapshot.interval` | specs/1-requirements/RQ-0502-配置管理需求.md:301 | specs/2-designs/DS-0502-配置管理设计.md:1020 | specs/3-tasks/TK-0102-实现存储引擎.md:244 |
| `storage.snapshot.threshold` | specs/1-requirements/RQ-0502-配置管理需求.md:302 | specs/2-designs/DS-0502-配置管理设计.md:1058 | specs/3-tasks/TK-0102-实现存储引擎.md:245 |
| `storage.snapshot.retention_count` | specs/1-requirements/RQ-0502-配置管理需求.md:303 | specs/2-designs/DS-0502-配置管理设计.md:1069 | specs/3-tasks/TK-0102-实现存储引擎.md:246 |
| `storage.snapshot.retention_days` | specs/1-requirements/RQ-0502-配置管理需求.md:304 | specs/2-designs/DS-0502-配置管理设计.md:1076 | specs/3-tasks/TK-0102-实现存储引擎.md:247 |
| `storage.badger.gc_interval` | specs/1-requirements/RQ-0502-配置管理需求.md:305 | specs/2-designs/DS-0502-配置管理设计.md:1027 | specs/3-tasks/TK-0403-实现嵌入式KV适配.md:26 |
| `storage.badger.gc_threshold` | specs/1-requirements/RQ-0502-配置管理需求.md:306 | specs/2-designs/DS-0502-配置管理设计.md:1033 | specs/3-tasks/TK-0403-实现嵌入式KV适配.md:26 |
| `storage.badger.cache_size` | specs/1-requirements/RQ-0502-配置管理需求.md:307 | specs/2-designs/DS-0502-配置管理设计.md:1083 | specs/3-tasks/TK-0403-实现嵌入式KV适配.md:27 |
| `session.ttl.default` | specs/1-requirements/RQ-0502-配置管理需求.md:313 | specs/2-designs/DS-0502-配置管理设计.md:1136 | specs/3-tasks/TK-0103-实现核心服务层.md:221 |
| `session.ttl.max` | specs/1-requirements/RQ-0502-配置管理需求.md:314 | specs/2-designs/DS-0502-配置管理设计.md:1137 | specs/3-tasks/TK-0103-实现核心服务层.md:222 |
| `session.ttl.gc_interval` | specs/1-requirements/RQ-0502-配置管理需求.md:315 | specs/2-designs/DS-0502-配置管理设计.md:1160 | specs/3-tasks/TK-0103-实现核心服务层.md:223 |
| `session.ttl.sample_size` | specs/1-requirements/RQ-0502-配置管理需求.md:316 | specs/2-designs/DS-0502-配置管理设计.md:1167 | specs/3-tasks/TK-0103-实现核心服务层.md:224 |
| `session.quota.max_per_user` | specs/1-requirements/RQ-0502-配置管理需求.md:317 | specs/2-designs/DS-0502-配置管理设计.md:1153 | specs/3-tasks/TK-0103-实现核心服务层.md:225 |
| `security.auth.rotation_grace` | specs/1-requirements/RQ-0502-配置管理需求.md:323 | specs/2-designs/DS-0502-配置管理设计.md:1216 | specs/3-tasks/TK-0201-实现安全与鉴权.md:138 |
| `security.auth.cache_capacity` | specs/1-requirements/RQ-0502-配置管理需求.md:324 | specs/2-designs/DS-0502-配置管理设计.md:1202 | specs/3-tasks/TK-0103-实现核心服务层.md:226 |
| `security.auth.cache_ttl` | specs/1-requirements/RQ-0502-配置管理需求.md:325 | specs/2-designs/DS-0502-配置管理设计.md:1209 | specs/3-tasks/TK-0103-实现核心服务层.md:227 |
| `security.auth.argon2.iterations` | specs/1-requirements/RQ-0502-配置管理需求.md:327 | specs/2-designs/DS-0502-配置管理设计.md:1231 | specs/3-tasks/TK-0201-实现安全与鉴权.md:195 |
| `security.auth.argon2.parallelism` | specs/1-requirements/RQ-0502-配置管理需求.md:328 | specs/2-designs/DS-0502-配置管理设计.md:1238 | specs/3-tasks/TK-0201-实现安全与鉴权.md:196 |
| `security.auth.allow_list` | specs/1-requirements/RQ-0502-配置管理需求.md:329 | specs/2-designs/DS-0502-配置管理设计.md:1184 | specs/3-tasks/TK-0201-实现安全与鉴权.md:191 |
| `security.anti_replay.timestamp_window` | specs/1-requirements/RQ-0502-配置管理需求.md:330 | specs/2-designs/DS-0502-配置管理设计.md:1247 | specs/3-tasks/TK-0103-实现核心服务层.md:230 |
| `security.anti_replay.nonce_ttl` | specs/1-requirements/RQ-0502-配置管理需求.md:331 | specs/2-designs/DS-0502-配置管理设计.md:1254 | specs/3-tasks/TK-0103-实现核心服务层.md:229 |
| `security.anti_replay.nonce_cache_size` | specs/1-requirements/RQ-0502-配置管理需求.md:332 | specs/2-designs/DS-0502-配置管理设计.md:1261 | specs/3-tasks/TK-0103-实现核心服务层.md:228 |
| `security.storage.wal_encryption_key` | specs/1-requirements/RQ-0502-配置管理需求.md:333 | specs/2-designs/DS-0502-配置管理设计.md:1271 | specs/3-tasks/TK-0201-实现安全与鉴权.md:199 |
| `telemetry.metrics.auth_enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:339 | specs/2-designs/DS-0502-配置管理设计.md:1546 | specs/3-tasks/TK-0001-Phase1-实施计划.md:99 |
| `telemetry.logging.level` | specs/1-requirements/RQ-0502-配置管理需求.md:340 | specs/2-designs/DS-0502-配置管理设计.md:339 | specs/3-tasks/TK-0303-实现管理接口.md:194 |
| `telemetry.logging.dump_config` | specs/1-requirements/RQ-0502-配置管理需求.md:341 | specs/2-designs/DS-0502-配置管理设计.md:1534 | specs/3-tasks/TK-0402-实现可观测性.md:28 |
| `telemetry.tracing.enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:342 | specs/2-designs/DS-0502-配置管理设计.md:1321 | specs/3-tasks/TK-0402-实现可观测性.md:30 |
| `telemetry.tracing.endpoint` | specs/1-requirements/RQ-0502-配置管理需求.md:343 | specs/2-designs/DS-0502-配置管理设计.md:1320 | specs/3-tasks/TK-0402-实现可观测性.md:30 |
| `telemetry.tracing.sampling_ratio` | specs/1-requirements/RQ-0502-配置管理需求.md:344 | specs/2-designs/DS-0502-配置管理设计.md:1326 | specs/3-tasks/TK-0402-实现可观测性.md:30 |
| `telemetry.audit.retention_days` | specs/1-requirements/RQ-0502-配置管理需求.md:345 | specs/2-designs/DS-0502-配置管理设计.md:1334 | specs/3-tasks/TK-0402-实现可观测性.md:31 |
| `server.http.enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:677 | specs/2-designs/DS-0502-配置管理设计.md:329 | specs/3-tasks/TK-0301-实现HTTP接口.md:28 |
| `server.http.max_body_size` | specs/1-requirements/RQ-0502-配置管理需求.md:679 | specs/2-designs/DS-0502-配置管理设计.md:733 | specs/3-tasks/TK-0301-实现HTTP接口.md:31 |
| `cluster.node_id` | specs/1-requirements/RQ-0502-配置管理需求.md:694 | specs/2-designs/DS-0502-配置管理设计.md:334 | specs/3-tasks/TK-0401-实现分布式集群.md:160 |
| `cluster.raft.snapshot_threshold` | specs/1-requirements/RQ-0502-配置管理需求.md:697 | specs/2-designs/DS-0502-配置管理设计.md:955 | specs/3-tasks/TK-0401-实现分布式集群.md:166 |
| `cluster.raft.trailing_logs` | specs/1-requirements/RQ-0502-配置管理需求.md:698 | specs/2-designs/DS-0502-配置管理设计.md:962 | specs/3-tasks/TK-0401-实现分布式集群.md:167 |
| `cluster.shutdown.force_leave` | specs/1-requirements/RQ-0502-配置管理需求.md:702 | specs/2-designs/DS-0502-配置管理设计.md:916 | specs/3-tasks/TK-0401-实现分布式集群.md:171 |
| `server.redis.enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:724 | specs/2-designs/DS-0502-配置管理设计.md:634 | specs/3-tasks/TK-0302-实现Redis协议.md:46 |
| `server.redis_tls.enabled` | specs/1-requirements/RQ-0502-配置管理需求.md:726 | specs/2-designs/DS-0502-配置管理设计.md:634 | specs/3-tasks/TK-0302-实现Redis协议.md:48 |
| `security.network.trusted_proxies` | specs/1-requirements/RQ-0502-配置管理需求.md:778 | specs/2-designs/DS-0502-配置管理设计.md:1194 | specs/3-tasks/TK-0201-实现安全与鉴权.md:191 |
