# Based on the deprecated `https://github.com/tutumcloud/tutum-debian`
FROM golang:1.14

# Use mirrors for poor network...
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list
RUN sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list

# Install packages
RUN apt-get update && \
    apt-get -y install \
        dos2unix \
        openssh-server \
        && \
mkdir -p /var/run/sshd && \
sed -i "s/UsePrivilegeSeparation.*/UsePrivilegeSeparation no/g" /etc/ssh/sshd_config && \
sed -i "s/PermitRootLogin without-password/PermitRootLogin yes/g" /etc/ssh/sshd_config

ENV AUTHORIZED_KEYS **None**

ADD ./bashrc /root/.bashrc
ADD run.sh /run.sh
RUN dos2unix /run.sh \
    && chmod +x /*.sh

# Install deps
RUN apt-get install -qqy sudo vim

EXPOSE 22
CMD ["/run.sh"]
