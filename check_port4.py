import paramiko

host = '185.190.140.62'
user = 'root'
password = 'm1etin123'

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=password)

# Check active listening ports
stdin, stdout, stderr = ssh.exec_command('ss -tulpn | grep litespeed || ss -tulpn | grep lshttpd || ps aux | grep litespeed')
print("SS output:")
print(stdout.read().decode())

ssh.close()
