drop database if exists `sharding1`;
drop database if exists `sharding2`;
create database `sharding1`;
use `sharding1`;
create table t1 (id bigint primary key, name varchar(80), info varchar(100)) DEFAULT CHARSET=utf8mb4;
create table t2 (id bigint primary key, name varchar(80), info varchar(100)) DEFAULT CHARSET=utf8mb4;
insert into t1 (id, name) values (10001, 'Gabriel García Márquez'), (10002, 'Cien años de soledad');
insert into t2 (id, name) values (20001, 'José Arcadio Buendía'), (20002, 'Úrsula Iguarán'), (20003, 'José Arcadio');
