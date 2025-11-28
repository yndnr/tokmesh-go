name: Bug 报告
description: 提交一个 TokMesh-Go 的缺陷报告
title: "[Bug] "
labels: ["bug"]
body:
  - type: textarea
    id: description
    attributes:
      label: 问题描述
      description: 请简要描述问题现象
      placeholder: 发生了什么？期望是什么？
    validations:
      required: true
  - type: textarea
    id: steps
    attributes:
      label: 复现步骤
      description: 提供尽量精确的复现步骤
      placeholder: |
        1. ...
        2. ...
        3. ...
    validations:
      required: true
  - type: textarea
    id: env
    attributes:
      label: 环境信息
      description: 包括 Go 版本、操作系统、TokMesh-Go 版本等
      placeholder: |
        - Go: 1.21
        - OS: ...
        - TokMesh-Go: ...

