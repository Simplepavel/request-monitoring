BEGIN;

CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE dormitories (
    id BIGSERIAL PRIMARY KEY,
    number INT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);



CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE ticket_statuses (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    is_final BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE status_transitions (
    id BIGSERIAL PRIMARY KEY,

    from_status_id BIGINT NOT NULL REFERENCES ticket_statuses(id) ON DELETE CASCADE,

    to_status_id BIGINT NOT NULL REFERENCES ticket_statuses(id) ON DELETE CASCADE,
    actor_role_id BIGINT REFERENCES roles(id) ON DELETE RESTRICT,
    comment_required BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    last_name VARCHAR(100) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    middle_name VARCHAR(100),
    phone VARCHAR(16) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE executor_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

    dormitory_id BIGINT NOT NULL REFERENCES dormitories(id) ON DELETE RESTRICT,
    position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    deactivated_at TIMESTAMPTZ
);

CREATE TABLE admin_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    dormitory_id BIGINT NOT NULL REFERENCES dormitories(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    deactivated_at TIMESTAMPTZ
);

CREATE TABLE super_admin_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    deactivated_at TIMESTAMPTZ
);

CREATE TABLE tickets (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    assignee_id BIGINT REFERENCES users(id) ON DELETE SET NULL,

    dormitory_id BIGINT NOT NULL
        REFERENCES dormitories(id) ON DELETE RESTRICT,

    block_number INT NOT NULL CHECK (
        block_number / 100 > 0 AND block_number % 100 BETWEEN 1 AND 99
    ),

    room_letter CHAR(1) CHECK (room_letter IN ('а', 'б')),

    location_description VARCHAR(255),

    position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,

    status_id BIGINT NOT NULL REFERENCES ticket_statuses(id) ON DELETE RESTRICT,

    title VARCHAR(255),
    description TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE TABLE ticket_status_history (
    id BIGSERIAL PRIMARY KEY,

    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    from_status_id BIGINT REFERENCES ticket_statuses(id),
    to_status_id BIGINT NOT NULL REFERENCES ticket_statuses(id),
    changed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,

    comment TEXT,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ticket_assignment_history (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    previous_assignee_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    new_assignee_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    changed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ticket_comments (
    id BIGSERIAL PRIMARY KEY,

    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ticket_photos (
    id BIGSERIAL PRIMARY KEY,

    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    file_path VARCHAR(500) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_author
    ON tickets(author_id);

CREATE INDEX idx_tickets_assignee
    ON tickets(assignee_id)
    WHERE assignee_id IS NOT NULL;

CREATE INDEX idx_tickets_status
    ON tickets(status_id);

CREATE INDEX idx_tickets_dorm_status
    ON tickets(dormitory_id, status_id);

CREATE INDEX idx_tsh_ticket
    ON ticket_status_history(ticket_id);

CREATE INDEX idx_tah_ticket
    ON ticket_assignment_history(ticket_id);

CREATE INDEX idx_comments_ticket
    ON ticket_comments(ticket_id);

CREATE INDEX idx_photos_ticket
    ON ticket_photos(ticket_id);

CREATE INDEX idx_user_roles_role
    ON user_roles(role_id);




INSERT INTO roles (name) VALUES
    ('resident'),
    ('executor'),
    ('admin'),
    ('super_admin');




INSERT INTO positions (name) VALUES
    ('plumber'), --  сантехник
    ('electrician'), --электрик
    ('locksmith'); --слесарь



INSERT INTO ticket_statuses (name, is_final) VALUES
    ('pending_review', FALSE),
    ('in_progress', FALSE),
    ('rejected_by_executor', FALSE),
    ('rejected_by_customer', FALSE),
    ('awaiting_confirmation', FALSE),
    ('completed', TRUE);




INSERT INTO status_transitions (
    from_status_id,
    to_status_id,
    actor_role_id,
    comment_required
)
SELECT
    f.id,
    t.id,
    r.id,
    v.comment_required
FROM (
    VALUES
        -- исполнитель принимает заявку
        (
            'pending_review',
            'in_progress',
            'executor',
            FALSE
        ),

        -- исполнитель завершил работу и ждёт подтверждения
        (
            'in_progress',
            'awaiting_confirmation',
            'executor',
            FALSE
        ),

        -- исполнитель отклоняет заявку с объяснением причины
        (
            'in_progress',
            'rejected_by_executor',
            'executor',
            TRUE
        ),

        -- житель подтверждает выполнение
        (
            'awaiting_confirmation',
            'completed',
            'resident',
            FALSE
        ),

        -- жителя не устроил результат + комментарий
        (
            'awaiting_confirmation',
            'rejected_by_customer',
            'resident',
            TRUE
        ),

        -- исполнитель ознакомился с замечаниями и начал доработку
        (
            'rejected_by_customer',
            'in_progress',
            'executor',
            FALSE
        ),

        -- житель исправил отклонённую заявку и отправил снова
        (
            'rejected_by_executor',
            'pending_review',
            'resident',
            FALSE
        )
) AS v(from_name, to_name, role_name, comment_required)
JOIN ticket_statuses f ON f.name = v.from_name
JOIN ticket_statuses t ON t.name = v.to_name
JOIN roles r ON r.name = v.role_name;




INSERT INTO status_transitions (
    from_status_id,
    to_status_id,
    actor_role_id,
    comment_required
)
SELECT
    f.id,
    t.id,
    NULL,
    FALSE
FROM (
    VALUES
        -- при деактивации исполнителя заявка возвращается в очередь
        (
            'in_progress',
            'pending_review'
        ),

        -- заявка на доработке тоже возвращается в очередь, если её исполнитель деактивирован
        (
            'rejected_by_customer',
            'pending_review'
        ),

        -- автозакрытие после истечения срока подтверждения
        (
            'awaiting_confirmation',
            'completed'
        )
) AS v(from_name, to_name)
JOIN ticket_statuses f ON f.name = v.from_name
JOIN ticket_statuses t ON t.name = v.to_name;




INSERT INTO dormitories (number) VALUES
    (10),
    (12),
    (13),
    (14),
    (15),
    (16),
    (21),
    (20),
    (22),
    (23);





COMMIT;