import paramiko

host = '185.190.140.62'
user = 'root'
password = 'm1etin123'

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=password)

stdin, stdout, stderr = ssh.exec_command('ps -p $(pgrep aurapanel) -o user=')
print("Service User:", stdout.read().decode().strip())
ssh.close()
