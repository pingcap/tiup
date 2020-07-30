FROM golang:1.14


# Use mirrors for poor network...
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list
RUN sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list


# tiup-cluster dependencies
 RUN apt-get -y -q update && \
     apt-get -y -q install software-properties-common && \
     apt-get install -qqy \
         dos2unix \
         default-mysql-client \
	     vim # not required by tiup-cluster itself, just for ease of use


# without --dev flag up.sh copies tiup-cluster to these subfolders
# with --dev flag they are empty until mounted
COPY tiup-cluster/tiup-cluster /tiup-cluster/tiup-cluster/
COPY tiup-cluster /tiup-cluster/

ADD bashrc /root/.bashrc
ADD init.sh /init.sh
RUN dos2unix /init.sh /root/.bashrc \
    && chmod +x /init.sh

# build tiup-cluster in without --dev flag
WORKDIR /tiup-cluster
RUN (test Makefile && make failpoint-enable && make cluster && make failpoint-disable) || true

CMD /init.sh
