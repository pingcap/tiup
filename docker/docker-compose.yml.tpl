version: "3.7"
x-node: &default-node
  build: ./node
  env_file: ./secret/node.env
  privileged: true
  networks:
    - tiops
  ports:
    - ${TIOPS_PORT:-22}

services:
  control:
    container_name: tiup-cluster-control
    hostname: control
    build: control
    env_file: ./secret/control.env
    privileged: true
    ports:
      - "8080"
    networks:
      tiops:
        ipv4_address: __IPPREFIX__.100
  n1:
    <<: *default-node
    container_name: tiup-cluster-n1
    hostname: n1
    # Uncomment for access grafana and prometheus from host when deploy them at n1.
    # networks:
    # ports:
    #   - "3000:3000"
    #   - "9090:9090"
    networks:
      tiops:
        ipv4_address: __IPPREFIX__.101
  n2:
    <<: *default-node
    container_name: tiup-cluster-n2
    hostname: n2
    networks:
      tiops:
        ipv4_address: __IPPREFIX__.102
  n3:
    <<: *default-node
    container_name: tiup-cluster-n3
    hostname: n3
    networks:
      tiops:
        ipv4_address: __IPPREFIX__.103
  n4:
    <<: *default-node
    container_name: tiup-cluster-n4
    hostname: n4
    networks:
      tiops:
        ipv4_address: __IPPREFIX__.104
  n5:
    <<: *default-node
    container_name: tiup-cluster-n5
    hostname: n5
    networks:
      tiops:
        ipv4_address: __IPPREFIX__.105

# docker network create --gateway __IPPREFIX__.1 --subnet __IPPREFIX__.0/24 tiup-cluster
networks:
  tiops:
    external: true
