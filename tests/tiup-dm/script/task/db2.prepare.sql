drop database if exists `sharding1`;
drop database if exists `sharding2`;
create database `sharding1`;
use `sharding1`;
create table t2 (id bigint primary key, name varchar(80), info varchar(100)) DEFAULT CHARSET=utf8mb4;
create table t3 (id bigint primary key, name varchar(80), info varchar(100)) DEFAULT CHARSET=utf8mb4;
insert into t2 (id, name, info) values (40000, 'Remedios Moscote', '{}');
insert into t3 (id, name, info) values (30001, 'Aureliano José', '{}'), (30002, 'Santa Sofía de la Piedad', '{}'), (30003, '17 Aurelianos', NULL);
