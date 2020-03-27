FROM golang:1.14


# Use mirrors for poor network...
RUN sed -i 's/archive.ubuntu.com/mirrors.aliyun.com/g' /etc/apt/sources.list
RUN sed -i 's/security.ubuntu.com/mirrors.aliyun.com/g' /etc/apt/sources.list
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list
RUN sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list


# tiops dependencies
 RUN apt-get -y -q update && \
     apt-get -y -q install software-properties-common && \
     apt-get install -qqy \
         dos2unix \
         git \
         wget \
         default-mysql-client \
	     vim pssh # not required by tiops itself, just for ease of use


# without --dev flag up.sh copies tiops to these subfolders
# with --dev flag they are empty until mounted
COPY tiops/tiops /tiops/tiops/
COPY tiops /tiops/

ADD ./bashrc /root/.bashrc
ADD ./init.sh /init.sh
RUN dos2unix /init.sh /root/.bashrc \
    && chmod +x /init.sh

# build tiops in without --dev flag
WORKDIR /tiops
RUN (test Makefile && make build) || true

CMD /init.sh
