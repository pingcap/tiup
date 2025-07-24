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
{% if ssh_proxy %}
  bastion:
    <<: *default-node
    container_name: tiup-cluster-bastion
    hostname: bastion
    networks:
      tiops:
        ipv4_address: {{ipprefix}}.250
      tiproxy:
        ipv4_address: {{proxy_prefix}}.250

{% for id in range(1, nodes+1) %}
  p{{id}}:
    <<: *default-node
    container_name: tiup-cluster-p{{id}}
    hostname: p{{id}}
    networks:
      tiproxy:
        ipv4_address: {{proxy_prefix}}.{{id+100}}
{% endfor %}
{% endif %}

networks:
  tiops:
    external: true
{% if ssh_proxy %}
  tiproxy:
    external: true
{% endif %}
