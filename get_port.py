import paramiko
import xml.etree.ElementTree as ET

host = '185.190.140.62'
user = 'root'
password = 'm1etin123'

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=password)

stdin, stdout, stderr = ssh.exec_command('cat /usr/local/lsws/admin/conf/admin_config.xml || cat /usr/local/lsws/admin/conf/admin_config.conf')
config_data = stdout.read().decode()
print("Config data length:", len(config_data))
lines = config_data.splitlines()
for line in lines:
    if "address" in line.lower() or "port" in line.lower() or "listener" in line.lower():
        print(line.strip())

ssh.close()
