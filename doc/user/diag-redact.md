# 本地 Diag Redact 入口

这个定制 TiUP 增加 `tiup diag-redact` 命令，直接运行 TiUP 可执行文件同目录里的 `diag-redact`。
它不注册镜像组件，不覆盖官方 Diag，也不更改其他组件的安装或调用规则。

将两个程序与 Diag 的 `info.toml`、模板一起放在独立目录，先使用明确路径测试：

```bash
./tiup diag-redact --version
./tiup diag-redact collect my-cluster --from '2026-10-30T11:00:00+08:00' --to '2026-10-30T11:01:00+08:00'
```

脱敏默认值由 `diag-redact` 实现，TiUP 只原样传递参数。
缺少本地程序时命令报错，不会从镜像下载或回退到官方 Diag。
进程直接替换，因此工作目录、环境、标准输入输出、信号和退出码由本地程序继承。

保留原有 TiUP 安装。若需要短命令，可以只在当前终端将独立目录放到 PATH 前面。
关闭终端后即可恢复原来的 PATH。普通 `tiup diag` 仍使用原组件解析流程。
该入口不支持通过 TiUP 镜像自动更新 `diag-redact`，更新时需更换测试包。
