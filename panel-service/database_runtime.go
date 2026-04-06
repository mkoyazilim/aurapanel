package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func sqlQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func commandOutputTrimmed(command string, args ...string) (string, error) {
	output, err := runCommandCombinedOutput(command, args...)
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func postgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func mysqlExec(query string) (string, error) {
	return commandOutputTrimmed("mysql", "-NBe", query)
}

func postgresExec(query string) (string, error) {
	return commandOutputTrimmed("runuser", "-u", "postgres", "--", "psql", "-tA", "-c", query)
}

func postgresExecDB(dbName, query string) (string, error) {
	dbName = sanitizeDBName(dbName)
	return commandOutputTrimmed("runuser", "-u", "postgres", "--", "psql", "-d", dbName, "-tA", "-c", query)
}

func buildTemporaryDBUsername(engine string) string {
	prefix := "apsso_"
	if normalizeEngine(engine) == "postgresql" {
		prefix = "apsso_pg_"
	}
	randomPart := strings.ToLower(strings.TrimSpace(generateSecret(5)))
	randomPart = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, randomPart)
	if randomPart == "" {
		randomPart = "user"
	}
	username := sanitizeDBName(prefix + randomPart)
	if len(username) > 30 {
		username = username[:30]
	}
	return username
}

func isRuntimeTemporaryDBUser(username string) bool {
	username = sanitizeDBName(username)
	if username == "" {
		return false
	}
	return strings.HasPrefix(username, "apsso_")
}

func createRuntimeTemporaryDBUser(engine, dbName, sourceUser string) (string, string, string, error) {
	engine = normalizeEngine(engine)
	dbName = sanitizeDBName(dbName)
	sourceUser = sanitizeDBName(sourceUser)
	if dbName == "" {
		return "", "", "", fmt.Errorf("database name is required for temporary db user")
	}

	username := buildTemporaryDBUsername(engine)
	password := generateSecret(18)
	host := "localhost"

	switch engine {
	case "mariadb":
		query := fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; FLUSH PRIVILEGES;",
			sqlQuote(username),
			sqlQuote(password),
			dbName,
			sqlQuote(username),
		)
		if _, err := mysqlExec(query); err != nil {
			return "", "", "", err
		}
	default:
		roleIdent := postgresIdentifier(username)
		if _, err := postgresExec(fmt.Sprintf("CREATE ROLE %s LOGIN INHERIT PASSWORD '%s';", roleIdent, sqlQuote(password))); err != nil {
			return "", "", "", err
		}
		if sourceUser != "" {
			if _, err := postgresExec(fmt.Sprintf("GRANT %s TO %s;", postgresIdentifier(sourceUser), roleIdent)); err != nil {
				_, _ = postgresExec(fmt.Sprintf("DROP ROLE IF EXISTS %s;", roleIdent))
				return "", "", "", err
			}
		}
		if _, err := postgresExec(fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;", postgresIdentifier(dbName), roleIdent)); err != nil {
			_, _ = postgresExec(fmt.Sprintf("DROP ROLE IF EXISTS %s;", roleIdent))
			return "", "", "", err
		}
		_, _ = postgresExecDB(dbName, fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s;", roleIdent))
		_, _ = postgresExecDB(dbName, fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s;", roleIdent))
		_, _ = postgresExecDB(dbName, fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s;", roleIdent))
		host = "127.0.0.1"
	}

	return username, password, host, nil
}

func dropRuntimeTemporaryDBUser(engine, username string) error {
	engine = normalizeEngine(engine)
	username = sanitizeDBName(username)
	if username == "" {
		return nil
	}
	if !isRuntimeTemporaryDBUser(username) {
		return nil
	}
	switch engine {
	case "mariadb":
		_, err := mysqlExec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'; FLUSH PRIVILEGES;", sqlQuote(username)))
		return err
	default:
		roleIdent := postgresIdentifier(username)
		_, _ = postgresExec(fmt.Sprintf("REASSIGN OWNED BY %s TO postgres;", roleIdent))
		_, _ = postgresExec(fmt.Sprintf("DROP OWNED BY %s;", roleIdent))
		_, err := postgresExec(fmt.Sprintf("DROP ROLE IF EXISTS %s;", roleIdent))
		return err
	}
}

func formatBytesHuman(value int64) string {
	if value >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(value)/(1024*1024*1024))
	}
	if value >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(value)/(1024*1024))
	}
	if value >= 1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	return fmt.Sprintf("%d B", value)
}

func runtimeDatabaseList(engine string) ([]DatabaseRecord, error) {
	switch engine {
	case "mariadb":
		out, err := mysqlExec(`SELECT s.schema_name, COALESCE(SUM(t.data_length + t.index_length),0), COUNT(t.table_name)
FROM information_schema.schemata s
LEFT JOIN information_schema.tables t ON t.table_schema = s.schema_name
WHERE s.schema_name NOT IN ('information_schema','mysql','performance_schema','sys')
GROUP BY s.schema_name
ORDER BY s.schema_name`)
		if err != nil {
			return nil, err
		}
		records := []DatabaseRecord{}
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			sizeBytes, _ := strconv.ParseInt(fields[1], 10, 64)
			tables, _ := strconv.Atoi(fields[2])
			records = append(records, DatabaseRecord{
				Name:   fields[0],
				Size:   formatBytesHuman(sizeBytes),
				Tables: tables,
				Engine: engine,
			})
		}
		return records, nil
	default:
		out, err := postgresExec(`SELECT d.datname, COALESCE(pg_database_size(d.datname),0)
FROM pg_database d
WHERE d.datistemplate = false AND d.datname <> 'postgres'
ORDER BY d.datname`)
		if err != nil {
			return nil, err
		}
		records := []DatabaseRecord{}
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			sizeBytes, _ := strconv.ParseInt(fields[1], 10, 64)
			records = append(records, DatabaseRecord{
				Name:   fields[0],
				Size:   formatBytesHuman(sizeBytes),
				Tables: 0,
				Engine: engine,
			})
		}
		return records, nil
	}
}

func runtimeDatabaseUsers(engine string) ([]DatabaseUser, error) {
	switch engine {
	case "mariadb":
		out, err := mysqlExec(`SELECT user, host FROM mysql.user WHERE user <> '' ORDER BY user, host`)
		if err != nil {
			return nil, err
		}
		users := []DatabaseUser{}
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			if isRuntimeTemporaryDBUser(fields[0]) {
				continue
			}
			users = append(users, DatabaseUser{Username: fields[0], Host: fields[1], Engine: engine})
		}
		return users, nil
	default:
		out, err := postgresExec(`SELECT rolname FROM pg_roles WHERE rolcanlogin = true AND rolname !~ '^pg_' ORDER BY rolname`)
		if err != nil {
			return nil, err
		}
		users := []DatabaseUser{}
		for _, line := range strings.Split(out, "\n") {
			role := strings.TrimSpace(line)
			if role == "" {
				continue
			}
			if isRuntimeTemporaryDBUser(role) {
				continue
			}
			users = append(users, DatabaseUser{Username: role, Host: "localhost", Engine: engine})
		}
		return users, nil
	}
}

func runtimeTemporaryDBUsers(engine string) ([]dbToolTempUser, error) {
	engine = normalizeEngine(engine)
	switch engine {
	case "mariadb":
		out, err := mysqlExec(`SELECT user FROM mysql.user WHERE user LIKE 'apsso_%' ORDER BY user`)
		if err != nil {
			return nil, err
		}
		users := []dbToolTempUser{}
		for _, line := range strings.Split(out, "\n") {
			username := sanitizeDBName(strings.TrimSpace(line))
			if !isRuntimeTemporaryDBUser(username) {
				continue
			}
			users = append(users, dbToolTempUser{Engine: engine, Username: username})
		}
		return users, nil
	default:
		out, err := postgresExec(`SELECT rolname FROM pg_roles WHERE rolname LIKE 'apsso_%' ORDER BY rolname`)
		if err != nil {
			return nil, err
		}
		users := []dbToolTempUser{}
		for _, line := range strings.Split(out, "\n") {
			username := sanitizeDBName(strings.TrimSpace(line))
			if !isRuntimeTemporaryDBUser(username) {
				continue
			}
			users = append(users, dbToolTempUser{Engine: engine, Username: username})
		}
		return users, nil
	}
}

func runtimeRemoteAccessList(engine string) ([]RemoteAccessRule, error) {
	switch engine {
	case "mariadb":
		out, err := mysqlExec(`SELECT user, host FROM mysql.user WHERE user <> '' AND host NOT IN ('localhost','127.0.0.1','::1') ORDER BY user, host`)
		if err != nil {
			return nil, err
		}
		rules := []RemoteAccessRule{}
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			rules = append(rules, RemoteAccessRule{Engine: engine, DBUser: fields[0], Remote: fields[1], AuthMethod: authMethodForEngine(engine)})
		}
		return rules, nil
	default:
		path := findPostgresHBAPath()
		if path == "" {
			return []RemoteAccessRule{}, nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rules := []RemoteAccessRule{}
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) < 5 || !strings.HasPrefix(fields[0], "host") {
				continue
			}
			rules = append(rules, RemoteAccessRule{
				Engine:     engine,
				DBName:     fields[1],
				DBUser:     fields[2],
				Remote:     fields[3],
				AuthMethod: fields[4],
			})
		}
		return rules, nil
	}
}

func createRuntimeDatabase(engine, dbName, dbUser, dbPass string) error {
	dbName = sanitizeDBName(dbName)
	dbUser = sanitizeDBName(dbUser)
	switch engine {
	case "mariadb":
		if _, err := mysqlExec(fmt.Sprintf(
			"CREATE DATABASE IF NOT EXISTS `%s`; CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; FLUSH PRIVILEGES;",
			dbName, sqlQuote(dbUser), sqlQuote(dbPass), dbName, sqlQuote(dbUser),
		)); err != nil {
			return err
		}
		return nil
	default:
		roleExists, err := postgresExec(fmt.Sprintf("SELECT 1 FROM pg_roles WHERE rolname = '%s';", sqlQuote(dbUser)))
		if err != nil {
			return err
		}
		roleIdent := postgresIdentifier(dbUser)
		if strings.TrimSpace(roleExists) == "1" {
			if _, err := postgresExec(fmt.Sprintf("ALTER ROLE %s WITH LOGIN PASSWORD '%s';", roleIdent, sqlQuote(dbPass))); err != nil {
				return err
			}
		} else {
			if _, err := postgresExec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s';", roleIdent, sqlQuote(dbPass))); err != nil {
				return err
			}
		}

		dbExists, err := postgresExec(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s';", sqlQuote(dbName)))
		if err != nil {
			return err
		}
		if strings.TrimSpace(dbExists) != "1" {
			if _, err := postgresExec(fmt.Sprintf("CREATE DATABASE %s OWNER %s;", postgresIdentifier(dbName), roleIdent)); err != nil {
				return err
			}
		}

		return nil
	}
}

func dropRuntimeDatabase(engine, dbName string) error {
	switch engine {
	case "mariadb":
		_, err := mysqlExec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName))
		return err
	default:
		_, _ = postgresExec(fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid();", sqlQuote(dbName)))
		_, err := postgresExec(fmt.Sprintf("DROP DATABASE IF EXISTS %s;", postgresIdentifier(dbName)))
		return err
	}
}

func updateRuntimeDatabasePassword(engine, dbUser, password string) error {
	switch engine {
	case "mariadb":
		out, err := mysqlExec(fmt.Sprintf("SELECT host FROM mysql.user WHERE user = '%s';", sqlQuote(dbUser)))
		if err != nil {
			return err
		}
		hosts := strings.Fields(strings.ReplaceAll(out, "\n", " "))
		if len(hosts) == 0 {
			hosts = []string{"localhost"}
		}
		for _, host := range hosts {
			if _, err := mysqlExec(fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s';", sqlQuote(dbUser), sqlQuote(host), sqlQuote(password))); err != nil {
				return err
			}
		}
		_, err = mysqlExec("FLUSH PRIVILEGES;")
		return err
	default:
		_, err := postgresExec(fmt.Sprintf("ALTER ROLE %s WITH PASSWORD '%s';", postgresIdentifier(dbUser), sqlQuote(password)))
		return err
	}
}

func grantRuntimeRemoteAccess(engine, dbUser, dbName, remoteIP string) error {
	switch engine {
	case "mariadb":
		out, err := mysqlExec(fmt.Sprintf("SELECT plugin, authentication_string FROM mysql.user WHERE user='%s' AND host='localhost' LIMIT 1;", sqlQuote(dbUser)))
		if err != nil {
			return err
		}
		fields := strings.Fields(out)
		plugin := "mysql_native_password"
		authString := ""
		if len(fields) >= 1 {
			plugin = fields[0]
		}
		if len(fields) >= 2 {
			authString = fields[1]
		}
		query := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s'", sqlQuote(dbUser), sqlQuote(remoteIP))
		if authString != "" {
			query += fmt.Sprintf(" IDENTIFIED WITH %s AS '%s'", plugin, sqlQuote(authString))
		}
		query += ";"
		query += fmt.Sprintf(" GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s'; FLUSH PRIVILEGES;", dbName, sqlQuote(dbUser), sqlQuote(remoteIP))
		_, err = mysqlExec(query)
		return err
	default:
		path := findPostgresHBAPath()
		if path == "" {
			return fmt.Errorf("pg_hba.conf not found")
		}
		line := fmt.Sprintf("host %s %s %s scram-sha-256", dbName, dbUser, remoteIP)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(raw), line) {
			content := strings.TrimRight(string(raw), "\n") + "\n" + line + "\n"
			if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
				return err
			}
		}
		_, err = commandOutputTrimmed("systemctl", "reload", "postgresql")
		return err
	}
}

func findPostgresHBAPath() string {
	candidates := []string{}
	if matches, _ := filepath.Glob("/etc/postgresql/*/main/pg_hba.conf"); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	candidates = append(candidates, "/var/lib/pgsql/data/pg_hba.conf")
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func (s *service) syncRuntimeDatabaseStateLocked(engine string) error {
	engine = normalizeEngine(engine)

	dbs, err := runtimeDatabaseList(engine)
	if err != nil {
		return err
	}
	users, err := runtimeDatabaseUsers(engine)
	if err != nil {
		return err
	}
	rules, err := runtimeRemoteAccessList(engine)
	if err != nil {
		return err
	}

	var existingDBs []DatabaseRecord
	var existingUsers []DatabaseUser
	if engine == "mariadb" {
		existingDBs = s.state.MariaDBs
		existingUsers = s.state.MariaUsers
	} else {
		existingDBs = s.state.PostgresDBs
		existingUsers = s.state.PostgresUsers
	}

	dbMeta := make(map[string]DatabaseRecord, len(existingDBs))
	for _, item := range existingDBs {
		dbMeta[item.Name] = item
	}
	for i := range dbs {
		if meta, ok := dbMeta[dbs[i].Name]; ok {
			dbs[i].Owner = meta.Owner
			dbs[i].SiteDomain = meta.SiteDomain
		}
	}

	userMeta := make(map[string]DatabaseUser, len(existingUsers))
	for _, item := range existingUsers {
		userMeta[item.Username] = item
	}
	for i := range users {
		if meta, ok := userMeta[users[i].Username]; ok {
			users[i].LinkedDBName = meta.LinkedDBName
			users[i].PasswordHash = meta.PasswordHash
		}
	}

	if engine == "mariadb" {
		s.state.MariaDBs = dbs
		s.state.MariaUsers = users
		s.state.MariaRemoteRules = rules
		return nil
	}

	s.state.PostgresDBs = dbs
	s.state.PostgresUsers = users
	s.state.PostgresRemoteRules = rules
	return nil
}
