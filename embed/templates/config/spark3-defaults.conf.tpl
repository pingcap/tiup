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

{{- define "PDList"}}
  {{- range $idx, $pd := .}}
    {{- if eq $idx 0}}
      {{- $pd}}
    {{- else -}}
      ,{{$pd}}
    {{- end}}
  {{- end}}
{{- end}}

{{ range $k, $v := .CustomFields}}
{{ $k }}   {{ $v }}
{{- end }}
spark.sql.extensions   org.apache.spark.sql.TiExtensions

{{- if .TiSparkMasters}}
spark.master   spark://{{.TiSparkMasters}}
{{- end}}

spark.tispark.pd.addresses {{template "PDList" .Endpoints}}

spark.sql.catalog.tidb_catalog  org.apache.spark.sql.catalyst.catalog.TiCatalog

spark.sql.catalog.tidb_catalog.pd.addresses  {{template "PDList" .Endpoints}}