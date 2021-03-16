# TiUP Cluster UI Changelog

## 2021.03.16

- Support show audit list

  ![audit list](https://user-images.githubusercontent.com/1284531/111284708-5760e780-867b-11eb-91d0-ea06b94ef203.png)
## 2021.03.15

- Support check a cluster before upgrading
- Support upgrade a cluster

## 2020.11.13

- Add audit for deploy/destroy/start/stop/scale_in/scale_out operations

## 2020.11.04

- Support config arch for machine

## 2020.11.03

- Use default "root" as the machine login user name if leave it empty
- Set default labels values if leave them empty
- Add confirmation prompt when starting to deploy

## 2020.10.26

- Enable manually edit the topo yaml configuration
- Support config the numa_node option for TiDB/TiKV/PD/TiFlash

## 2020.10.21

- Skip check TiKV location labels when deploying or scaling out to enable deploy multiple TiKV instances in a same host

## 2020.10.16

- Add data management and db users management features by embeding TiDB dashboard
- Add full TiDB dashboard features entry

## 2020.10.15

- Support to modify mirror address

## 2020.10.14

- Support to modify cluster configuration by embeding TiDB dashboard

## 2020.09.16

- Enable select TiDB v4.0.6 to deploy, support type any TiDB version manually
