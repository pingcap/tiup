# Based on the deprecated `https://github.com/tutumcloud/tutum-debian`
FROM golang:1.17-bullseye

# Use mirrors for poor network...
#RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list && \
#    sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list

# Install packages
# JRE 11 is installed for tispark testing, a tispark node could be started
# with Java 11, but is not going to work properly. However, we don't test
# for SQL in CI, so it's ok to use Java 11 instead of Java 8.
RUN apt-get update && \
    apt-get -y install \
        dos2unix \
        openssh-server \
        openjdk-11-jre-headless \
        sudo vim \
        iproute2 \
        && \
    mkdir -p /var/run/sshd && \
    sed -i "s/UsePrivilegeSeparation.*/UsePrivilegeSeparation no/g" /etc/ssh/sshd_config && \
    sed -i "s/PermitRootLogin without-password/PermitRootLogin yes/g" /etc/ssh/sshd_config

ENV AUTHORIZED_KEYS **None**

ADD bashrc /root/.bashrc
ADD run.sh /run.sh
RUN dos2unix /run.sh \
    && chmod +x /*.sh

EXPOSE 22
CMD ["/run.sh"]
