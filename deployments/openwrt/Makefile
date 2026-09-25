include $(TOPDIR)/rules.mk

PKG_NAME:=gsm2mqtt
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/legoser/gsm2mqtt.git
PKG_SOURCE_VERSION:=v$(PKG_VERSION)
PKG_MIRROR_HASH:=skip

PKG_LICENSE:=Apache-2.0
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=GSM2MQTT Authors

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

GO_PKG:=github.com/legoser/gsm2mqtt
GO_PKG_BUILD_PKG:=github.com/legoser/gsm2mqtt/cmd/gsm2mqtt
GO_PKG_LDFLAGS_X:=main.version=v$(PKG_VERSION) main.buildTime=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

include $(INCLUDE_DIR)/package.mk
include ../../lang/golang/golang-package.mk

define Package/gsm2mqtt
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Telephony
  TITLE:=GSM to MQTT gateway (SMS, USSD, modem management)
  URL:=https://github.com/legoser/gsm2mqtt
  DEPENDS:=$(GO_ARCH_DEPENDS)
endef

define Package/gsm2mqtt/description
  GSM2MQTT connects GSM modems (serial/USB) to MQTT broker, providing
  SMS send/receive, USSD execution, and modem monitoring capabilities.
endef

define Package/gsm2mqtt/conffiles
/etc/config/gsm2mqtt
/etc/gsm2mqtt/gsm2mqtt.yaml
endef

define Package/gsm2mqtt/install
	$(call GoPackage/Package/Install/Bin,$(1))
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/gsm2mqtt.init $(1)/etc/init.d/gsm2mqtt
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/gsm2mqtt.config $(1)/etc/config/gsm2mqtt
	$(INSTALL_DIR) $(1)/etc/gsm2mqtt
	$(INSTALL_CONF) ./files/gsm2mqtt.yaml $(1)/etc/gsm2mqtt/gsm2mqtt.yaml
endef

define Package/gsm2mqtt/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] || {
	/etc/init.d/gsm2mqtt enable
}
exit 0
endef

define Package/gsm2mqtt/prerm
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] || {
	/etc/init.d/gsm2mqtt disable
	/etc/init.d/gsm2mqtt stop
}
exit 0
endef

$(eval $(call GoBinPackage,gsm2mqtt))
$(eval $(call BuildPackage,gsm2mqtt))
