const http = require('http');
const fs = require('fs');
const path = require('path');

const server = http.createServer((req, res) => {
    let filePath = 'D:/Projeler/aurapanel-ols/internal/webui/dist' + req.url;
    if (filePath.endsWith('/')) filePath += 'index.html';
    
    fs.readFile(filePath, (error, content) => {
        if (error) {
            console.error("404:", filePath);
            res.writeHead(404);
            res.end('Not found');
        } else {
            let extname = path.extname(filePath);
            let contentType = 'text/html';
            if (extname === '.js') contentType = 'text/javascript';
            else if (extname === '.css') contentType = 'text/css';
            
            res.writeHead(200, { 'Content-Type': contentType });
            res.end(content, 'utf-8');
        }
    });
});

server.listen(8085, '127.0.0.1', () => {
    console.log('Server running at http://127.0.0.1:8085/');
});