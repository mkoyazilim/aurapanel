<?php
declare(strict_types=1);

header('Content-Type: application/json; charset=utf-8');
header('X-Content-Type-Options: nosniff');

if ($_SERVER['REQUEST_METHOD'] === 'OPTIONS') {
    http_response_code(204);
    exit;
}

if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    respond(405, 'error', 'Method not allowed.');
}

function respond(int $code, string $status, string $message, array $extra = []): void
{
    http_response_code($code);
    echo json_encode(array_merge([
        'status' => $status,
        'message' => $message,
    ], $extra), JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    exit;
}

function envFilePath(): string
{
    $path = getenv('AURAPANEL_COMMUNITY_ENV_PATH');
    if (is_string($path) && trim($path) !== '') {
        return trim($path);
    }
    $candidates = [
        '/home/aurapanel.info/.config/community-site.env',
        '/etc/aurapanel/community-site.env',
    ];
    foreach ($candidates as $candidate) {
        if (is_file($candidate)) {
            return $candidate;
        }
    }
    return $candidates[0];
}

function loadEnvFile(string $path): array
{
    if (!is_file($path)) {
        return [];
    }

    $lines = file($path, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
    if ($lines === false) {
        return [];
    }

    $values = [];
    foreach ($lines as $line) {
        $line = trim($line);
        if ($line === '' || str_starts_with($line, '#') || !str_contains($line, '=')) {
            continue;
        }
        [$k, $v] = explode('=', $line, 2);
        $values[trim($k)] = trim($v);
    }
    return $values;
}

function configValue(array $fileEnv, string $key, string $fallback = ''): string
{
    $fromProcess = getenv($key);
    if (is_string($fromProcess) && trim($fromProcess) !== '') {
        return trim($fromProcess);
    }
    if (array_key_exists($key, $fileEnv) && trim((string)$fileEnv[$key]) !== '') {
        return trim((string)$fileEnv[$key]);
    }
    return $fallback;
}

function clientIp(): string
{
    $xff = $_SERVER['HTTP_X_FORWARDED_FOR'] ?? '';
    if (is_string($xff) && trim($xff) !== '') {
        $parts = explode(',', $xff);
        if (count($parts) > 0) {
            return trim($parts[0]);
        }
    }
    $remote = $_SERVER['REMOTE_ADDR'] ?? '';
    return is_string($remote) ? trim($remote) : '';
}

function allowedOriginCheck(array $fileEnv): bool
{
    $raw = configValue($fileEnv, 'AURAPANEL_COMMUNITY_ALLOWED_ORIGINS', 'https://aurapanel.info,https://www.aurapanel.info');
    $allowed = array_filter(array_map('trim', explode(',', $raw)), static fn(string $v): bool => $v !== '');
    if (count($allowed) === 0) {
        return true;
    }

    $origin = $_SERVER['HTTP_ORIGIN'] ?? '';
    if (is_string($origin) && trim($origin) !== '') {
        return in_array(trim($origin), $allowed, true);
    }

    $referer = $_SERVER['HTTP_REFERER'] ?? '';
    if (!is_string($referer) || trim($referer) === '') {
        return false;
    }
    foreach ($allowed as $item) {
        if (str_starts_with($referer, $item)) {
            return true;
        }
    }
    return false;
}

function inputPayload(): array
{
    $raw = file_get_contents('php://input');
    if (!is_string($raw) || trim($raw) === '') {
        return [];
    }
    $decoded = json_decode($raw, true);
    return is_array($decoded) ? $decoded : [];
}

function sanitizeText(string $value, int $maxLen): string
{
    $value = trim($value);
    if ($value === '') {
        return '';
    }
    if (mb_strlen($value) > $maxLen) {
        $value = mb_substr($value, 0, $maxLen);
    }
    return $value;
}

function ensureRateLimit(string $ip, array $fileEnv): void
{
    if ($ip === '') {
        return;
    }

    $maxAttempts = (int)configValue($fileEnv, 'AURAPANEL_COMMUNITY_RATE_LIMIT_MAX', '4');
    $windowSeconds = (int)configValue($fileEnv, 'AURAPANEL_COMMUNITY_RATE_LIMIT_WINDOW', '900');
    $stateFile = configValue($fileEnv, 'AURAPANEL_COMMUNITY_RATE_LIMIT_FILE', '/tmp/aurapanel-community-rate-limit.json');

    if ($maxAttempts < 1) {
        $maxAttempts = 4;
    }
    if ($windowSeconds < 60) {
        $windowSeconds = 900;
    }

    $key = hash('sha256', $ip);
    $now = time();
    $state = [];

    if (is_file($stateFile)) {
        $raw = file_get_contents($stateFile);
        $decoded = is_string($raw) ? json_decode($raw, true) : null;
        if (is_array($decoded)) {
            $state = $decoded;
        }
    }

    $entries = [];
    if (isset($state[$key]) && is_array($state[$key])) {
        foreach ($state[$key] as $ts) {
            $ts = (int)$ts;
            if ($ts > 0 && ($now - $ts) <= $windowSeconds) {
                $entries[] = $ts;
            }
        }
    }

    if (count($entries) >= $maxAttempts) {
        respond(429, 'error', 'Too many requests. Please try again later.');
    }

    $entries[] = $now;
    $state[$key] = $entries;

    @file_put_contents($stateFile, json_encode($state, JSON_UNESCAPED_SLASHES), LOCK_EX);
}

$fileEnv = loadEnvFile(envFilePath());
if (!allowedOriginCheck($fileEnv)) {
    respond(403, 'error', 'Invalid origin.');
}

$payload = inputPayload();

$fullName = sanitizeText((string)($payload['full_name'] ?? ''), 120);
$email = strtolower(sanitizeText((string)($payload['email'] ?? ''), 190));
$company = sanitizeText((string)($payload['company'] ?? ''), 190);
$role = sanitizeText((string)($payload['role'] ?? ''), 50);
$focus = sanitizeText((string)($payload['focus'] ?? ''), 2000);
$websiteTrap = sanitizeText((string)($payload['website_url'] ?? ''), 250);

if ($websiteTrap !== '') {
    // Silent success for bot traffic.
    respond(200, 'success', 'Request submitted.');
}

if ($fullName === '' || mb_strlen($fullName) < 2) {
    respond(422, 'error', 'Please provide a valid full name.');
}

if ($email === '' || !filter_var($email, FILTER_VALIDATE_EMAIL)) {
    respond(422, 'error', 'Please provide a valid email.');
}

$allowedRoles = ['operator', 'agency', 'developer', 'reseller'];
if (!in_array($role, $allowedRoles, true)) {
    respond(422, 'error', 'Please select a valid role.');
}

if ($focus === '' || mb_strlen($focus) < 8) {
    respond(422, 'error', 'Please provide a short focus area.');
}

$ip = clientIp();
ensureRateLimit($ip, $fileEnv);

$dbHost = configValue($fileEnv, 'AURAPANEL_COMMUNITY_DB_HOST', '127.0.0.1');
$dbPort = configValue($fileEnv, 'AURAPANEL_COMMUNITY_DB_PORT', '3306');
$dbName = configValue($fileEnv, 'AURAPANEL_COMMUNITY_DB_NAME', 'aurapanel_community');
$dbUser = configValue($fileEnv, 'AURAPANEL_COMMUNITY_DB_USER', 'aurapanel_community');
$dbPass = configValue($fileEnv, 'AURAPANEL_COMMUNITY_DB_PASS', '');

if ($dbPass === '') {
    respond(500, 'error', 'Community intake database is not configured.');
}

$dsn = sprintf('mysql:host=%s;port=%s;dbname=%s;charset=utf8mb4', $dbHost, $dbPort, $dbName);

try {
    $pdo = new PDO($dsn, $dbUser, $dbPass, [
        PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
        PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
    ]);

    $pdo->exec(
        'CREATE TABLE IF NOT EXISTS community_signups (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            full_name VARCHAR(120) NOT NULL,
            email VARCHAR(190) NOT NULL,
            company VARCHAR(190) NOT NULL DEFAULT "",
            role VARCHAR(50) NOT NULL,
            focus TEXT NOT NULL,
            source_page VARCHAR(190) NOT NULL DEFAULT "",
            user_agent VARCHAR(255) NOT NULL DEFAULT "",
            ip_hash CHAR(64) NOT NULL,
            status VARCHAR(30) NOT NULL DEFAULT "new",
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE KEY uniq_email_role (email, role)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci'
    );

    $stmt = $pdo->prepare(
        'INSERT INTO community_signups (
            full_name, email, company, role, focus, source_page, user_agent, ip_hash
         ) VALUES (
            :full_name, :email, :company, :role, :focus, :source_page, :user_agent, :ip_hash
         )
         ON DUPLICATE KEY UPDATE
            company = VALUES(company),
            focus = VALUES(focus),
            source_page = VALUES(source_page),
            user_agent = VALUES(user_agent),
            ip_hash = VALUES(ip_hash),
            status = "updated"'
    );

    $sourcePage = '';
    $referer = $_SERVER['HTTP_REFERER'] ?? '';
    if (is_string($referer) && trim($referer) !== '') {
        $sourcePage = sanitizeText($referer, 190);
    }
    $userAgent = sanitizeText((string)($_SERVER['HTTP_USER_AGENT'] ?? ''), 255);
    $ipHash = hash('sha256', $ip !== '' ? $ip : 'unknown');

    $stmt->execute([
        ':full_name' => $fullName,
        ':email' => $email,
        ':company' => $company,
        ':role' => $role,
        ':focus' => $focus,
        ':source_page' => $sourcePage,
        ':user_agent' => $userAgent,
        ':ip_hash' => $ipHash,
    ]);
} catch (Throwable $e) {
    respond(500, 'error', 'Request could not be persisted right now.');
}

respond(200, 'success', 'Request submitted successfully.');
