Vagrant.configure("2") do |config|
  ssh_pub_key = File.readlines("#{File.dirname(__FILE__)}/vagrant_key.pub").first.strip

  config.vm.box = "hashicorp/bionic64"
  config.vm.provision "shell", privileged: false, inline: <<-SHELL
    sudo apt install -y zsh
    sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
    sudo chsh -s /usr/bin/zsh vagrant
    echo #{ssh_pub_key} >> /home/vagrant/.ssh/authorized_keys
  SHELL

  config.vm.provision "shell", privileged: true, inline: <<-SHELL
    echo "setting ulimit"
    sudo echo "fs.file-max = 65535" >> /etc/sysctl.conf
    sudo sysctl -p
    sudo echo "*               hard    nofile          65535" >> /etc/security/limits.conf
    sudo echo "*               soft    nofile          65535" >> /etc/security/limits.conf
    sudo echo "root            hard    nofile          65535" >> /etc/security/limits.conf
    sudo echo "root            hard    nofile          65535" >> /etc/security/limits.conf
  SHELL
end

# ulimit ref: https://my.oschina.net/u/914655/blog/3067520
