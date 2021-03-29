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
        ipv4_address: {{ipprefix}}.100
{% for id in range(1, nodes+1) %}
  n{{id}}:
    <<: *default-node
    container_name: tiup-cluster-n{{id}}
    hostname: n{{id}}
    networks:
      tiops:
        ipv4_address: {{ipprefix}}.{{id+100}}
{% endfor %}

# docker network create --gateway {{ipprefix}}.1 --subnet {{ipprefix}}.0/24 tiup-cluster
networks:
  tiops:
    external: true
