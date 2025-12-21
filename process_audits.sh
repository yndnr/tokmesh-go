#!/bin/bash
# 批量处理审核报告脚本

PENDING_DIR="specs/audits/pending"
PROCESSED_DIR="specs/audits/processed"
SUMMARY_FILE="audit_summary.md"

mkdir -p "$PROCESSED_DIR"

echo "# 审核报告处理摘要" > "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"
echo "生成时间: $(date '+%Y-%m-%d %H:%M:%S')" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

total=0
critical=0
warnings=0
suggestions=0

echo "## 报告统计" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

for file in "$PENDING_DIR"/*.md; do
    if [ -f "$file" ]; then
        total=$((total + 1))
        basename=$(basename "$file" _audit.md)

        # 统计问题数量
        crit=$(grep -c "\[严重\]" "$file" 2>/dev/null || echo 0)
        warn=$(grep -c "\[警告\]" "$file" 2>/dev/null || echo 0)
        sugg=$(grep -c "\[建议\]" "$file" 2>/dev/null || echo 0)

        critical=$((critical + crit))
        warnings=$((warnings + warn))
        suggestions=$((suggestions + sugg))

        # 输出有问题的报告
        if [ $crit -gt 0 ] || [ $warn -gt 0 ]; then
            echo "- **$basename**: 严重=$crit, 警告=$warn, 建议=$sugg" >> "$SUMMARY_FILE"
        fi
    fi
done

echo "" >> "$SUMMARY_FILE"
echo "## 总计" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"
echo "- 总报告数: $total" >> "$SUMMARY_FILE"
echo "- 严重问题: $critical" >> "$SUMMARY_FILE"
echo "- 警告问题: $warnings" >> "$SUMMARY_FILE"
echo "- 建议项: $suggestions" >> "$SUMMARY_FILE"

cat "$SUMMARY_FILE"
