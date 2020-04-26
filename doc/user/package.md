# Component packaging

When you want to add a new component, or add a version of an existing component, you need to use tar to package the relevant file and then pass it to the mirror repository, using tar to package is not a difficult thing, the trouble is that you need to update the repository's meta-information, to avoid updating meta-information when corrupting the information of existing components. So the package component takes on this task.

```bash
[root@localhost ~]# tiup package --help
Package a tiup component and generate package directory

Usage:
  tiup package target [flags]

Flags:
  -C, -- string          打 tar 包前先切换目录，等同于 tar 的 -C 参数
      --arch string      组件运行的处理器架构 (默认为当前的 GOARCH)
      --desc string      组件的描述信息
      --entry string     组件的二进制文件位于包中的位置
  -h, --help             帮助信息
      --hide tiup list   在 tiup list 中隐藏该组件
      --name string      组件名称
      --os string        组件运行的操作系统 (默认为当前的 GOOS)
      --release string   组件的版本
      --standalone       该组件是否可以独立运行（例如 PD 不能独立运行，但是 playground 可以）
```

## Hello World

In this section we develop and package a hello component whose only function is to output the contents of its own configuration file, which is called "Hello World". For the sake of simplicity, we use the bash script to develop this component.

1. first create its configuration file, which contains only "Hello World":

```shell
cat > config.txt << EOF
Hello World
EOF
```

2. The executable is then created:

```shell
cat > hello.sh << EOF
#! /bin/sh
cat \${TIUP_COMPONENT_INSTALL_DIR}/config.txt
EOF

chmod 755 hello.sh
```

The environment variable `TIUP_COMPONENT_INSTALL_DIR` is passed in by TiUP at runtime and points to the installation directory of the component.

3. Then refer to [Build Private Mirror] (. /mirrors.md) to build an offline or private mirror (mainly because the official mirror is not available to publish). /mirrors.md) to build an offline or private mirror (mainly because the official mirror is not available to publish its own package), and make sure the TIUP_MIRRORS variable points to the built mirror.

4. Wrapping:

```shell
tiup package hello.sh config.txt --name=hello --entry=hello.sh --release=v0.0.1
```

This step creates a package directory where the packaged files and meta information are placed.

5. Upload to the warehouse
   
   Since the official repository is not currently open for uploading, we can only upload to our own mirrors built in step 3, by copying all the files in the package directly to ${target-dir} in step 3 tiup mirrors.

```bash
cp package/* path/to/mirror/
```

If the directory created in step 3 happens to be in the current directory and is called package, then there is no need to manually copy it.

6. Implementation

```bash
[root@localhost ~]# tiup list hello --refresh
Available versions for hello (Last Modified: 2020-04-23T16:45:53+08:00):
Version  Installed  Release:                   Platforms
-------  ---------  --------                   ---------
v0.0.1              2020-04-23T16:51:41+08:00  darwin/amd64

[root@localhost ~]# tiup hello
The component `hello` is not installed; downloading from repository.
Starting component `hello`: /Users/joshua/.tiup/components/hello/v0.0.1/hello.sh
Hello World
```
