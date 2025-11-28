name: 功能需求
description: 提出一个新的功能或改进建议
title: "[Feature] "
labels: ["enhancement"]
body:
  - type: textarea
    id: motivation
    attributes:
      label: 背景与动机
      description: 这个需求要解决什么问题？
    validations:
      required: true
  - type: textarea
    id: proposal
    attributes:
      label: 大致方案
      description: 描述你期望的行为或接口形态（不需要到实现细节）
    validations:
      required: true
  - type: textarea
    id: extra
    attributes:
      label: 其他补充信息
      description: 例如相关的 SDD 文档、R 编号、已有讨论链接等

