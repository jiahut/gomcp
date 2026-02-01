gomcp 管理脚本

使用方法:
  /home/zhijia/.local/bin/gomcpman [command]

服务管理命令:
  status      查看服务状态
  logs        实时查看日志 (按 Ctrl+C 退出)
  start       启动服务
  stop        停止服务
  restart     重启服务
  enable      启用开机自启
  disable     禁用开机自启
  install     安装服务到用户 systemd
  uninstall   卸载服务

浏览器容器管理命令:
  update-browser   拉取最新镜像（仅在镜像更新时重新创建容器）
  browser-status   查看浏览器容器状态
  browser-logs     查看浏览器容器日志 (按 Ctrl+C 退出)

其他命令:
  help        显示此帮助信息

示例:
  /home/zhijia/.local/bin/gomcpman status         # 查看当前服务状态
  /home/zhijia/.local/bin/gomcpman logs           # 实时查看日志
  /home/zhijia/.local/bin/gomcpman restart        # 重启服务
  /home/zhijia/.local/bin/gomcpman enable         # 启用开机自启
  /home/zhijia/.local/bin/gomcpman update-browser # 更新浏览器容器

