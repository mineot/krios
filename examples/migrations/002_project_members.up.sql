CREATE TABLE project_members (
	project_id INTEGER NOT NULL REFERENCES projects(id),
	user_id INTEGER NOT NULL,
	role TEXT NOT NULL,
	PRIMARY KEY (project_id, user_id)
);
