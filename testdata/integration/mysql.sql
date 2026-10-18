CREATE DATABASE IF NOT EXISTS otra;
USE pluton;
CREATE TABLE clientes (
  id    INT AUTO_INCREMENT PRIMARY KEY,
  email VARCHAR(100) NOT NULL,
  edad  INT DEFAULT 18,
  alta  DATETIME DEFAULT CURRENT_TIMESTAMP,
  bin   VARBINARY(4),
  UNIQUE KEY ux_clientes_email (email),
  KEY ix_clientes_edad_alta (edad, alta)
);
CREATE VIEW v_clientes AS SELECT id, email FROM clientes;
INSERT INTO clientes (email, edad, bin) VALUES ('a@x.com', 30, 0xFF00), ('b@x.com', NULL, NULL);
CREATE TABLE otra.t (id INT PRIMARY KEY);
GRANT ALL ON otra.* TO 'pluton'@'%';
