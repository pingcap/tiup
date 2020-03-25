FROM ubuntu:16.04


# Use mirrors for poor network...
RUN sed -i 's/archive.ubuntu.com/mirrors.aliyun.com/g' /etc/apt/sources.list
RUN sed -i 's/security.ubuntu.com/mirrors.aliyun.com/g' /etc/apt/sources.list


# tiops dependencies
 RUN apt-get -y -q update && \
     apt-get -y -q install software-properties-common && \
     apt-get install -qqy \
         dos2unix \
         git \
         wget \
	     vim pssh # not required by tiops itself, just for ease of use

RUN wget https://dl.google.com/go/go1.14.1.linux-amd64.tar.gz && \
	tar -C /usr/local -xzf go1.14.1.linux-amd64.tar.gz

# without --dev flag up.sh copies tiops to these subfolders
# with --dev flag they are empty until mounted
COPY tiops/tiops /tiops/tiops/
COPY tiops /tiops/

ADD ./bashrc /root/.bashrc
ADD ./init.sh /init.sh
RUN dos2unix /init.sh /root/.bashrc \
    && chmod +x /init.sh

CMD /init.sh
