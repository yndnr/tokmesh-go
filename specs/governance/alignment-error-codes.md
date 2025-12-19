# 错误码对齐矩阵（规范 / 需求 / 设计 / 任务）

**状态**: 草稿
**最后更新**: 2025-12-18

单一事实来源：`specs/governance/error-codes.md`

## 阻断问题：引用但未定义

- `TM-ADMIN-4092`

| 错误码 | 需求引用(首处) | 设计引用(首处) | 任务引用(首处) |
|---|---|---|---|
| `TM-ADMIN-4030` | 无 | specs/2-designs/DS-0301-接口与协议层设计.md:171 | specs/3-tasks/TK-0303-实现管理接口.md:42 |
| `TM-ADMIN-4031` | 无 | specs/2-designs/DS-0302-管理接口设计.md:1038 | specs/3-tasks/TK-0303-实现管理接口.md:57 |
| `TM-ADMIN-4041` | 无 | specs/2-designs/DS-0302-管理接口设计.md:1039 | specs/3-tasks/TK-0303-实现管理接口.md:330 |
| `TM-ADMIN-4091` | 无 | specs/2-designs/DS-0302-管理接口设计.md:1040 | specs/3-tasks/TK-0303-实现管理接口.md:331 |
| `TM-ADMIN-4130` | specs/1-requirements/RQ-0304-管理接口规约.md:139 | specs/2-designs/DS-0302-管理接口设计.md:528 | specs/3-tasks/TK-0301-实现HTTP接口.md:31 |
| `TM-ADMIN-4221` | 无 | specs/2-designs/DS-0302-管理接口设计.md:1042 | specs/3-tasks/TK-0303-实现管理接口.md:333 |
| `TM-ADMIN-4291` | 无 | specs/2-designs/DS-0302-管理接口设计.md:1043 | specs/3-tasks/TK-0303-实现管理接口.md:334 |
| `TM-ADMIN-5001` | 无 | specs/2-designs/DS-0302-管理接口设计.md:1044 | specs/3-tasks/TK-0303-实现管理接口.md:335 |
| `TM-ADMIN-5031` | specs/1-requirements/RQ-0304-管理接口规约.md:138 | specs/2-designs/DS-0302-管理接口设计.md:532 | specs/3-tasks/TK-0303-实现管理接口.md:336 |
| `TM-ARG-1001` | specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md:91 | specs/2-designs/DS-0103-核心服务层设计.md:123 | specs/3-tasks/TK-0604-实现CLI-apikey命令.md:318 |
| `TM-ARG-1002` | 无 | specs/2-designs/DS-0301-接口与协议层设计.md:173 | 无 |
| `TM-ARG-1003` | 无 | specs/2-designs/DS-0301-接口与协议层设计.md:174 | 无 |
| `TM-AUTH-4010` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:672 | specs/3-tasks/TK-0101-实现核心数据模型.md:129 |
| `TM-AUTH-4011` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:676 | specs/3-tasks/TK-0101-实现核心数据模型.md:130 |
| `TM-AUTH-4012` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:674 | specs/3-tasks/TK-0101-实现核心数据模型.md:131 |
| `TM-AUTH-4013` | 无 | 无 | 无 |
| `TM-AUTH-4014` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:599 | specs/3-tasks/TK-0101-实现核心数据模型.md:132 |
| `TM-AUTH-4015` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:607 | 无 |
| `TM-AUTH-4030` | specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md:101 | specs/2-designs/DS-0103-核心服务层设计.md:756 | specs/3-tasks/TK-0101-实现核心数据模型.md:133 |
| `TM-AUTH-4031` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:679 | specs/3-tasks/TK-0101-实现核心数据模型.md:134 |
| `TM-CFG-1001` | specs/1-requirements/RQ-0502-配置管理需求.md:355 | specs/2-designs/DS-0502-配置管理设计.md:672 | 无 |
| `TM-CFG-1002` | specs/1-requirements/RQ-0502-配置管理需求.md:356 | specs/2-designs/DS-0302-管理接口设计.md:792 | 无 |
| `TM-CFG-1003` | specs/1-requirements/RQ-0502-配置管理需求.md:357 | specs/2-designs/DS-0502-配置管理设计.md:863 | 无 |
| `TM-CFG-1004` | 无 | specs/2-designs/DS-0502-配置管理设计.md:867 | 无 |
| `TM-CFG-1005` | 无 | specs/2-designs/DS-0502-配置管理设计.md:712 | 无 |
| `TM-CFG-1006` | specs/1-requirements/RQ-0502-配置管理需求.md:358 | specs/2-designs/DS-0502-配置管理设计.md:653 | 无 |
| `TM-CFG-1007` | 无 | specs/2-designs/DS-0502-配置管理设计.md:877 | 无 |
| `TM-CFG-1008` | 无 | 无 | 无 |
| `TM-CLI-1001` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:596 | 无 |
| `TM-CLI-1002` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:597 | 无 |
| `TM-CLI-1003` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:598 | 无 |
| `TM-CLI-1004` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:599 | 无 |
| `TM-CLI-1005` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:600 | 无 |
| `TM-CLI-1006` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:601 | 无 |
| `TM-CLI-1007` | 无 | specs/2-designs/DS-0601-CLI总体设计.md:602 | 无 |
| `TM-CLST-4090` | 无 | specs/2-designs/DS-0401-分布式集群架构设计.md:278 | 无 |
| `TM-CLST-5030` | 无 | specs/2-designs/DS-0401-分布式集群架构设计.md:275 | 无 |
| `TM-CLST-5040` | 无 | specs/2-designs/DS-0401-分布式集群架构设计.md:276 | 无 |
| `TM-CLST-5050` | 无 | specs/2-designs/DS-0401-分布式集群架构设计.md:277 | 无 |
| `TM-SESS-4001` | specs/1-requirements/RQ-0101-核心数据模型.md:131 | specs/2-designs/DS-0301-接口与协议层设计.md:163 | specs/3-tasks/TK-0101-实现核心数据模型.md:119 |
| `TM-SESS-4002` | specs/1-requirements/RQ-0102-会话生命周期管理.md:173 | specs/2-designs/DS-0103-核心服务层设计.md:126 | specs/3-tasks/TK-0001-Phase1-实施计划.md:208 |
| `TM-SESS-4040` | specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md:153 | specs/2-designs/DS-0103-核心服务层设计.md:124 | specs/3-tasks/TK-0101-实现核心数据模型.md:114 |
| `TM-SESS-4041` | specs/1-requirements/RQ-0102-会话生命周期管理.md:186 | specs/2-designs/DS-0103-核心服务层设计.md:289 | specs/3-tasks/TK-0101-实现核心数据模型.md:115 |
| `TM-SESS-4090` | 无 | specs/2-designs/DS-0301-接口与协议层设计.md:161 | specs/3-tasks/TK-0101-实现核心数据模型.md:117 |
| `TM-SESS-4091` | 无 | specs/2-designs/DS-0102-存储引擎设计.md:1085 | specs/3-tasks/TK-0001-Phase1-实施计划.md:207 |
| `TM-SYS-4000` | specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md:78 | specs/2-designs/DS-0301-接口与协议层设计.md:175 | specs/3-tasks/TK-0301-实现HTTP接口.md:267 |
| `TM-SYS-4290` | specs/1-requirements/RQ-0201-安全与鉴权体系.md:116 | specs/2-designs/DS-0103-核心服务层设计.md:777 | specs/3-tasks/TK-0201-实现安全与鉴权.md:127 |
| `TM-SYS-5000` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:127 | 无 |
| `TM-SYS-5001` | 无 | specs/2-designs/DS-0102-存储引擎设计.md:1080 | 无 |
| `TM-SYS-5002` | 无 | specs/2-designs/DS-0102-存储引擎设计.md:1083 | 无 |
| `TM-SYS-5030` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:375 | 无 |
| `TM-TOKN-4000` | 无 | specs/2-designs/DS-0301-接口与协议层设计.md:165 | specs/3-tasks/TK-0101-实现核心数据模型.md:122 |
| `TM-TOKN-4010` | specs/1-requirements/RQ-0102-会话生命周期管理.md:78 | specs/2-designs/DS-0103-核心服务层设计.md:124 | specs/3-tasks/TK-0101-实现核心数据模型.md:123 |
| `TM-TOKN-4011` | specs/1-requirements/RQ-0102-会话生命周期管理.md:79 | specs/2-designs/DS-0301-接口与协议层设计.md:167 | specs/3-tasks/TK-0101-实现核心数据模型.md:124 |
| `TM-TOKN-4012` | specs/1-requirements/RQ-0102-会话生命周期管理.md:80 | specs/2-designs/DS-0301-接口与协议层设计.md:168 | specs/3-tasks/TK-0101-实现核心数据模型.md:125 |
| `TM-TOKN-4090` | 无 | specs/2-designs/DS-0103-核心服务层设计.md:904 | specs/3-tasks/TK-0101-实现核心数据模型.md:126 |
