# TokMesh 代码审核报告目录

**工作流版本**: v2.0
**最后更新**: 2025-12-21

---

## 📁 目录结构

```
specs/audits/
├── approved/          # ✅ 审核通过（无严重问题，7天后自动删除）
├── pending/           # ⚠️ 待修复（有严重问题或警告需修复）
├── fixed/             # 🔄 已修复，待复核（复核后立即删除）
├── review.log         # 复核通过记录（仅日志）
├── scripts/           # 自动化脚本
│   ├── audit_all.sh       # 执行代码审核
│   ├── fix_all.sh         # 执行问题修复
│   ├── review_all.sh      # 执行修复复核
│   └── cleanup_approved.sh # 清理过期报告（7天）
├── audit_progress.md  # 审核进度总览
└── MIGRATION_V2.md    # v2.0 迁移记录
```

**关键变化** (v2.1):
- ✅ **复核通过** → 立即删除，仅记录日志（不再归档到 approved/）
- ❌ **复核不通过** → 更新报告，移回 pending/（不再使用 rejected/ 目录）

---

## 🎯 分类标准

| 结论 | 条件 | 目录 | 保留期限 |
|------|------|------|----------|
| ✅ **审核通过** | 严重=0 **且** 警告≤3 **且** 评分≥85 | `approved/` | 7天后自动删除 |
| ⚠️ **需修复** | 其他情况 | `pending/` | 修复后移至 `fixed/` |
| 🔄 **待复核** | 问题已修复 | `fixed/` | **复核后立即删除** |
| ✅ **复核通过** | 所有问题已解决 | ~~无~~ (删除) | **立即删除，仅记录日志** |
| ❌ **复核不通过** | 修复不彻底 | `pending/` | 更新报告，等待下轮修复 |

---

## 📊 当前状态

**审核进度**: 4/775 个核心文件已审核 (~0.5%)

### approved/ (2个文件)

| 文件 | 评分 | 问题 | 状态 |
|------|------|------|------|
| `token.go` | 88/100 | 警告2个, 建议3个 | ✅ 质量优秀 |
| `apikey.go` | 85/100 | 警告3个, 建议4个 | ✅ 质量良好 |

### pending/ (2个文件)

| 文件 | 评分 | 严重问题 | 状态 |
|------|------|----------|------|
| `errors.go` | 78/100 | 1个（引用完整性） | ⚠️ 需修复 |
| `session.go` | 82/100 | 2个（参数校验） | ⚠️ 需修复 |

---

## 🚀 使用指南

### 1. 执行审核

```bash
# 全量审核（自动分类到 approved/ 或 pending/）
cd specs/audits/scripts
./audit_all.sh

# 指定模块审核
./audit_all.sh ../../../src/internal/core/service/
```

### 2. 修复问题

```bash
# 修复 pending/ 中的所有问题
./fix_all.sh

# 修复指定文件
./fix_all.sh 2025-12-21_internal-core-domain-errors_pending.md
```

### 3. 复核修复

```bash
# 复核 fixed/ 中的所有修复
./review_all.sh
```

### 4. 清理过期报告

```bash
# 手动清理
./cleanup_approved.sh

# 设置定时任务（每天凌晨2点）
crontab -e
# 添加: 0 2 * * * /home/yangsen/codes/tokmesh/specs/audits/scripts/cleanup_approved.sh
```

---

## 📖 参考文档

- [audit-workflow-v2.md](../governance/audit-workflow-v2.md) - 完整工作流规范
- [audit-framework.md](../governance/audit-framework.md) - 审核标准和维度
- [MIGRATION_V2.md](MIGRATION_V2.md) - v2.0 迁移记录

---

## 🔄 工作流图

```
代码模块 → 审核
           ↓
      ┌────┴────┐
      ↓         ↓
   ✅通过    ⚠️需修复
   approved/  pending/
   (7天删除)     ↓
                修复
                 ↓
              fixed/
            (临时存放)
                 ↓
              复核
           ┌────┴────┐
           ↓         ↓
        ✅通过    ❌不通过
       立即删除   更新报告
      (记日志)   移回pending/
```

---

**维护**: 自动清理 + 手动归档
**更新**: 实时（每次审核后更新进度）
