import paramiko

host = '185.190.140.62'
user = 'root'
password = 'm1etin123'

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=password)

# Check active listening ports
stdin, stdout, stderr = ssh.exec_command('netstat -tulpn | grep -E "7080|17080"')
print("Netstat 7080/17080:")
print(stdout.read().decode())

ssh.close()
