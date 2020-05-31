# Component packaging

When you want to add a new component, or add a version of an existing component, you need to use tar to package the relevant file and then pass it to the mirror repository, using tar to package is not a difficult thing, the trouble is that you need to update the repository's meta-information, to avoid updating meta-information when corrupting the information of existing components. So the package component takes on this task.

```bash
[user@localhost ~]# tiup package --help
Package a tiup component and generate package directory

Usage:
  tiup package target [flags]

Flags:
  -C, -- string          Change directory before compress
      --arch string      Target ARCH of the package (default "amd64")
      --desc string      Description of the package
      --entry string     Entry point of the package
  -h, --help             help for tiup
      --hide tiup list   Don't show the component in tiup list
      --name string      Name of the package
      --os string        Target OS of the package (default "darwin")
      --release string   Version of the package
      --standalone       Can the component run standalone
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
[user@localhost ~]# tiup list hello
Available versions for hello (Last Modified: 2020-04-23T16:45:53+08:00):
Version  Installed  Release:                   Platforms
-------  ---------  --------                   ---------
v0.0.1              2020-04-23T16:51:41+08:00  darwin/amd64

[user@localhost ~]# tiup hello
The component `hello` is not installed; downloading from repository.
Starting component `hello`: /Users/joshua/.tiup/components/hello/v0.0.1/hello.sh
Hello World
```
