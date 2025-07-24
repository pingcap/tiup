FROM golang:1.21

# tiup-cluster dependencies
 RUN apt-get -y -q update && \
     apt-get -y -q install software-properties-common && \
     apt-get install -qqy \
         dos2unix \
         default-mysql-client \
         psmisc \
         vim # not required by tiup-cluster itself, just for ease of use


WORKDIR /tiup-cluster
# without --dev flag up.sh copies tiup-cluster to these subfolders
# with --dev flag they are empty until mounted
# COPY .er/tiup-cluster /tiup-cluster/tiup-cluster/
COPY . /tiup-cluster/
RUN (test Makefile && make failpoint-enable && make cluster && make failpoint-disable) || true

ADD bashrc /root/.bashrc
ADD init.sh /init.sh
RUN dos2unix /init.sh /root/.bashrc && \
    chmod +x /init.sh && \
    mkdir -p /root/.ssh && \
    echo "Host *\n    ServerAliveInterval 30\n    ServerAliveCountMax 3" >> /root/.ssh/config

CMD /init.sh
