const { execSync } = require('child_process');
const http = require('http');
const fs = require('fs');
const path = require('path');

const server = http.createServer((req, res) => {
    let filePath = 'D:/Projeler/aurapanel-ols/internal/webui/dist' + req.url;
    if (filePath.endsWith('/')) filePath += 'index.html';
    
    let extname = path.extname(filePath);
    let contentType = 'text/html';
    switch (extname) {
        case '.js': contentType = 'text/javascript'; break;
        case '.css': contentType = 'text/css'; break;
        case '.json': contentType = 'application/json'; break;
        case '.png': contentType = 'image/png'; break;
    }
    
    fs.readFile(filePath, (error, content) => {
        if (error) {
            res.writeHead(500);
            res.end('Error: ' + error.code + ' ..\n');
        } else {
            res.writeHead(200, { 'Content-Type': contentType });
            res.end(content, 'utf-8');
        }
    });
});

server.listen(8083, '127.0.0.1', () => {
    console.log('Server running at http://127.0.0.1:8083/');
});