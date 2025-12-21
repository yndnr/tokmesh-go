#!/bin/bash
# 自动清理 approved/ 中超过 7 天的审核报告

APPROVED_DIR="specs/audits/approved"
DAYS_TO_KEEP=7

echo "====== 审核报告自动清理 ======"
echo "清理目录: $APPROVED_DIR"
echo "保留期限: $DAYS_TO_KEEP 天"
echo ""

# 统计待清理文件
OLD_FILES=$(find "$APPROVED_DIR" -name "*.md" -mtime +$DAYS_TO_KEEP 2>/dev/null)
COUNT=$(echo "$OLD_FILES" | grep -c "\.md" 2>/dev/null || echo 0)

if [ $COUNT -eq 0 ]; then
    echo "✅ 无需清理（没有超过 $DAYS_TO_KEEP 天的报告）"
    exit 0
fi

echo "📋 发现 $COUNT 个超期报告："
echo "$OLD_FILES" | while read -r file; do
    if [ -n "$file" ]; then
        filename=$(basename "$file")
        mtime=$(stat -c %y "$file" 2>/dev/null | cut -d' ' -f1)
        echo "  - $filename (修改时间: $mtime)"
    fi
done

echo ""
echo "🗑️ 开始清理..."

# 删除超期文件
find "$APPROVED_DIR" -name "*.md" -mtime +$DAYS_TO_KEEP -delete

echo "✅ 清理完成：已删除 $COUNT 个报告"
echo ""
echo "====== 清理日志 ======"
echo "$(date '+%Y-%m-%d %H:%M:%S') - 清理 $COUNT 个报告" >> specs/audits/cleanup.log
