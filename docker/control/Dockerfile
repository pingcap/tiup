FROM golang:1.24

WORKDIR /tiup-cluster
COPY . /tiup-cluster/
COPY my.cnf /root/.my.cnf

# tiup-cluster dependencies
 RUN apt-get -y -q update && \
     apt-get install -qqy \
         dos2unix \
         default-mysql-client \
         psmisc \
         vim # not required by tiup-cluster itself, just for ease of use

ADD bashrc /root/.bashrc
ADD init.sh /init.sh
RUN dos2unix /init.sh /root/.bashrc && \
    chmod +x /init.sh && \
    mkdir -p /root/.ssh && \
    echo "Host *\n    ServerAliveInterval 30\n    ServerAliveCountMax 3" >> /root/.ssh/config

CMD /init.sh
