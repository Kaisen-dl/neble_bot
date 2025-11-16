-- init.sql
CREATE TABLE IF NOT EXISTS user_roles (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(20) NOT NULL,
    user_name VARCHAR(100) NOT NULL,
    role_id VARCHAR(20) NOT NULL,
    role_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    renewal_status VARCHAR(20) DEFAULT 'pending',
    message_id VARCHAR(20) DEFAULT ''
);

SELECT rolname, rolpassword IS NOT NULL as has_password FROM pg_catalog.pg_roles;

CREATE INDEX IF NOT EXISTS idx_user_roles_expires_at ON user_roles(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);