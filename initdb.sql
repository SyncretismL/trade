CREATE TABLE public.users (
    id bigserial PRIMARY KEY,
    firstname text NOT NULL,
    lastname text NOT NULL,
    email text UNIQUE NOT NULL,
    password text NOT NULL,
    birthday text,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL
);

CREATE TABLE public.sessions (
    token text NOT NULL,
    user_id bigserial PRIMARY KEY,
    created_at timestamp NOT NULL,
    valid_until timestamp NOT NULL,
    FOREIGN KEY (user_id) REFERENCES public.users(id)
);

CREATE TABLE public.robots (
    id bigserial PRIMARY KEY,
    owner_user_id integer NOT NULL,
    parent_robot_id integer,
    is_favorite boolean NOT NULL,
    is_active boolean NOT NULL,
    ticker text NOT NULL,
    buy_price numeric(5, 2) NOT NULL,
    sell_price numeric(5, 2) NOT NULL,
    plan_start timestamp NOT NULL,
    plan_end timestamp NOT NULL,
    plan_yield integer NOT NULL,
    fact_yield integer,
    deals_count integer NOT NULL,
    activated_at timestamp,
    deactivated_at timestamp,
    created_at timestamp NOT NULL,
    deleted_at timestamp
);
    -- FOREIGN KEY (owner_user_id) REFERENCES public.users(id)
    -- FOREIGN KEY (parent_robot_id) REFERENCES public.robots(id)


INSERT INTO public.posts (title, description, price) VALUES ('post3', 'desc3', 110.99);

INSERT INTO public.photos1
VALUES (1, 'ref1', 1),
       (2, 'ref2', 1),
       (3, 'ref1', 2),
       (4, 'ref1', 2),
       (5, 'ref1', 2);