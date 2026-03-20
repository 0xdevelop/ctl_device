package ctl_device

import "github.com/0xdevelop/ctl_device/config"

// Version [☑]Option
/*
	en: Get `ctl_device` version;
	zh-CN: 获取`ctl_device`版本;
	@return [☑]string en: version string;zh-CN: 版本字符串;
*/
func Version() string {
	return config.ProjectVersion
}
