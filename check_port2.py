import paramiko

host = '185.190.140.62'
user = 'root'
password = 'm1etin123'

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=password)

# Check active listening ports for litespeed
stdin, stdout, stderr = ssh.exec_command('netstat -tulpn | grep litespeed')
print("Netstat output:")
print(stdout.read().decode())

# Check config again
stdin, stdout, stderr = ssh.exec_command('grep -i -A 2 "adminListener" /usr/local/lsws/admin/conf/admin_config.conf')
print("Config output:")
print(stdout.read().decode())

ssh.close()
