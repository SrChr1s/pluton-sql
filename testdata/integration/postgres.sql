CREATE SCHEMA ventas;
CREATE TABLE public.clientes (
  id    serial PRIMARY KEY,
  email text NOT NULL,
  edad  int DEFAULT 18,
  alta  timestamptz DEFAULT now()
);
CREATE UNIQUE INDEX ux_clientes_email ON public.clientes (email);
CREATE INDEX ix_clientes_edad_alta ON public.clientes (edad, alta);
CREATE VIEW public.v_clientes AS SELECT id, email FROM public.clientes;
CREATE TABLE ventas.pedidos (id bigserial PRIMARY KEY, cliente_id int NOT NULL, total numeric(10,2));
INSERT INTO public.clientes (email, edad) VALUES ('a@x.com', 30), ('b@x.com', NULL);
