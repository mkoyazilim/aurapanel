import paramiko
import time

host = '185.190.140.62'
user = 'root'
password = 'm1etin123'
local_file = 'aurapanel_linux_amd64'
remote_tmp = '/tmp/aurapanel_tmp'
remote_file = '/usr/local/sbin/aurapanel'

print("Connecting...")
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=password)

print("Stopping service...")
ssh.exec_command('systemctl stop aurapanel')
time.sleep(2)

print("Uploading to /tmp...")
sftp = ssh.open_sftp()
sftp.put(local_file, remote_tmp)
sftp.close()

print("Moving binary and setting permissions...")
ssh.exec_command(f'mv -f {remote_tmp} {remote_file}')
ssh.exec_command(f'chmod 755 {remote_file}')

print("Starting service...")
ssh.exec_command('systemctl start aurapanel')
time.sleep(2)

stdin, stdout, stderr = ssh.exec_command('systemctl is-active aurapanel')
status = stdout.read().decode().strip()
print("Service status:", status)

ssh.close()
print("Deploy complete!")
