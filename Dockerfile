FROM golang:latest
COPY ./ /src/
WORKDIR /src
RUN make