# AMLLHub
新一代AMLL歌词站，与AMLL歌词生态协作

[AMLL TTML 歌词站](https://amlldb.bikonoo.com) 的重构版本

## 架构
用户层:     React + Vite + TypeScript 

接入层:     Nginx

认证层:     Casdoor

网关层:     Go (Gin) + JWT

业务层:     Go + GORM

搜索层:     MeiliSearch

数据层:     PostgreSQL + Redis + MinIO + RabbitMQ
