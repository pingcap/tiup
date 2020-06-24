#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Default system properties included when running spark-submit.
# This is useful for setting default environmental settings.

# Example:
#spark.eventLog.dir: "hdfs://namenode:8021/directory"
# spark.executor.extraJavaOptions  -XX:+PrintGCDetails -Dkey=value -Dnumbers="one two three"

{% for item, value in spark_defaults_custom | dictsort -%}
{{ item }}   {{ value }}
{% endfor %}

{% set tispark_master = [] -%}
{% set tispark_master_hosts = groups.spark_master %}
{% for host in tispark_master_hosts -%}
  {% set tispark_master_ip = hostvars[host].ansible_host | default(hostvars[host].inventory_hostname) -%}
  {% set _ = tispark_master.append("%s:%s" % (tispark_master_ip, '7077')) -%}
{% endfor -%}

{% if tispark_master %}
spark.master   spark://{{ tispark_master | join('') }}
{% endif %}

{% set all_pd = [] -%}
{% for host in groups.pd_servers -%}
  {% set other_ip = hostvars[host].ansible_host | default(hostvars[host].inventory_hostname) -%}
  {% set other_port = hostvars[host]['pd_client_port'] -%}
  {% set _ = all_pd.append("%s:%s" % (other_ip, other_port)) -%}
{% endfor -%}

spark.tispark.pd.addresses   {{ all_pd | join(',') }}
