# Развёртывание GSM2MQTT на OpenWrt

В данном документе описывается установка, настройка и сборка пакетов **GSM2MQTT** для маршрутизаторов под управлением **OpenWrt**.

Поддерживаются оба пакетных менеджера:
- **OPKG (`.ipk`)** — стандартный пакетный менеджер для OpenWrt 23.05 и более ранних версий.
- **APK (`.apk`)** — современный пакетный менеджер на базе `apk-tools`, используемый начиная с OpenWrt 25.12 и snapshot-версий.

---

## Поддерживаемые архитектуры

| Архитектура Go | Пакет OPKG | Пакет APK | Примеры устройств |
|---|---|---|---|
| `amd64` | `x86_64` | `x86_64` | x86/x64 мини-ПК, виртуальные машины, x86_64 роутеры |
| `arm64` | `aarch64_cortex-a53`<br>`aarch64_cortex-a72`<br>`aarch64_generic` | `aarch64` | Raspberry Pi 3 (`cortex-a53`), Raspberry Pi 4 (`cortex-a72`), NanoPi R2S/R4S/R5S, GL.iNet MT3000 / Flint 2 |
| `arm` (ARMv7) | `arm_cortex-a7_neon-vfpv4`<br>`arm_cortex-a9`<br>`arm_cortex-a15_neon-vfpv4` | `armv7` | Роутеры на Cortex-A7/A9 (IPQ40xx, BCM53xx, Marvell Armada) |
| `mipsle` | `mipsel_24kc` | `mipsel` | MediaTek MT7621, MT7628, GL.iNet Mango/Shadow |
| `mips` | `mips_24kc` | `mips` | Atheros AR9331, AR9344, QCA9531 |
| `riscv64` | `riscv64` | `riscv64` | RISC-V платы и роутеры |

> **Подсказка по выбору пакета OPKG:**
> Узнайте точное имя архитектуры вашего устройства командой:
> ```sh
> opkg print-architecture
> ```
> Для **Raspberry Pi 3** это `aarch64_cortex-a53`, для **Raspberry Pi 4** — `aarch64_cortex-a72`, для x86_64 — `x86_64`.

---

## 1. Подготовка OpenWrt к работе с модемами

Для доступа к USB/Serial GSM-модемам установите драйверы ядра:

```sh
# Обновление списка пакетов
opkg update
# Или в OpenWrt 25+:
# apk update

# Необходимые драйверы USB Serial
opkg install kmod-usb-serial kmod-usb-serial-option kmod-usb-acm usbutils
# Или в OpenWrt 25+:
# apk add kmod-usb-serial kmod-usb-serial-option kmod-usb-acm usbutils
```

После подключения модема проверьте наличие устройства:
```sh
ls -la /dev/ttyUSB* /dev/ttyACM*
```

---

## 2. Установка через OPKG (.ipk)

Для OpenWrt 23.05 и более ранних версий:

1. Скачайте `.ipk` пакет нужной архитектуры из раздела [Releases](https://github.com/legoser/gsm2mqtt/releases) (например, для Raspberry Pi 3: `gsm2mqtt_1.0.0-1_aarch64_cortex-a53.ipk`).
2. Скопируйте файл на роутер через `scp`:
   ```sh
   scp gsm2mqtt_1.0.0-1_aarch64_cortex-a53.ipk root@192.168.1.1:/tmp/
   ```
3. Установите пакет (обязательно указывайте полный путь `/tmp/...` или `./...`):
   ```sh
   opkg install /tmp/gsm2mqtt_1.0.0-1_aarch64_cortex-a53.ipk
   ```
   > **Важно:** Если вы скачали пакет `aarch64_generic.ipk` и хотите установить его на архитектуру Cortex-A53 без пересборки, используйте флаг `--force-architecture`:
   > ```sh
   > opkg install --force-architecture /tmp/gsm2mqtt_1.0.0-1_aarch64_generic.ipk
   > ```

Пакет автоматически создаст и включит службу автозапуска `procd`.

---

## 3. Установка через APK (.apk)

Для OpenWrt 25.12 и snapshot-сборок:

1. Скачайте `.apk` пакет нужной архитектуры из раздела [Releases](https://github.com/legoser/gsm2mqtt/releases) (например, `gsm2mqtt-1.0.0-r1-x86_64.apk`).
2. Скопируйте файл на роутер через `scp`:
   ```sh
   scp gsm2mqtt-1.0.0-r1-x86_64.apk root@192.168.1.1:/tmp/
   ```
3. Установите пакет:
   ```sh
   apk add --allow-untrusted /tmp/gsm2mqtt-1.0.0-r1-x86_64.apk
   ```

---

## 4. Конфигурация

Пакет устанавливает следующие файлы:
- `/usr/bin/gsm2mqtt` — исполняемый бинарный файл.
- `/etc/init.d/gsm2mqtt` — init-скрипт для службы `procd`.
- `/etc/config/gsm2mqtt` — UCI-конфигурация OpenWrt.
- `/etc/gsm2mqtt/gsm2mqtt.yaml` — основной файл настроек (защищён от перезаписи при обновлении).

### UCI конфигурация (`/etc/config/gsm2mqtt`)

Позволяет быстро включить или отключить службу и указать путь к конфигурационному файлу:

```uci
config gsm2mqtt 'config'
	option enabled '1'
	option config_file '/etc/gsm2mqtt/gsm2mqtt.yaml'
```

### Основная конфигурация (`/etc/gsm2mqtt/gsm2mqtt.yaml`)

Отредактируйте параметры подключения к вашему MQTT-брокеру и модему:

```yaml
log_level: info

mqtt:
  broker: "192.168.1.100"  # Адрес MQTT брокера
  port: 1883
  username: ""
  password: ""
  topic_prefix: "gsm2mqtt"
  discovery: true

modems:
  - id: "modem1"
    name: "OpenWrt Modem"
    port: "/dev/ttyUSB0"
    baud_rate: 115200
    type: "auto"
    pin: ""
```

---

## 5. Управление службой (procd)

Служба управляется стандартными командами `rc.common` / `procd`:

```sh
# Запуск
/etc/init.d/gsm2mqtt start

# Остановка
/etc/init.d/gsm2mqtt stop

# Перезапуск
/etc/init.d/gsm2mqtt restart

# Проверка статуса
/etc/init.d/gsm2mqtt status

# Просмотр логов в реальном времени
logread -f -e gsm2mqtt
```

Служба запускается под суперпользователем `root` (что обеспечивает прямой доступ к `/dev/ttyUSB*`) с опцией автоматического перезапуска (`respawn`) при сбоях.

---

## 6. Локальная сборка пакетов

В исходном коде проекта доступны готовые сценарии для сборки пакетов без необходимости разворачивать полный OpenWrt SDK:

```sh
# Собрать все пакеты (OPKG и APK для всех архитектур)
make package-openwrt

# Собрать только OPKG (.ipk)
make package-opkg

# Собрать только APK (.apk)
make package-apk
```

Скрипт `scripts/build_openwrt_packages.sh` позволяет собирать пакеты выборочно:

```sh
# Сборка для конкретной архитектуры
./scripts/build_openwrt_packages.sh --arch amd64 --type all --version v1.0.0 --out-dir dist

# Сборка только APK для ARM64
./scripts/build_openwrt_packages.sh --arch arm64 --type apk --version v1.0.0 --out-dir dist
```

> **Примечание:** Для сборки APK используется утилита `apk mkpkg`. Если она не установлена локально в системе, скрипт автоматически использует легковесный контейнер `alpine:latest` через Docker.

---

## 7. Сборка через OpenWrt SDK / Feed

В каталоге `deployments/openwrt/` находится стандартный `Makefile` пакета для включения в дерево исходников OpenWrt или пользовательский feed:

1. Скопируйте каталог в feed или `package/` вашего OpenWrt buildroot / SDK:
   ```sh
   cp -r deployments/openwrt /path/to/openwrt/package/gsm2mqtt
   ```
2. Выберите пакет в `make menuconfig` (раздел `Network -> Telephony -> gsm2mqtt`).
3. Скомпилируйте:
   ```sh
   make package/gsm2mqtt/compile V=s
   ```
В зависимости от версии SDK будет автоматически сгенерирован `.ipk` (в OpenWrt <=24) или `.apk` (в OpenWrt >=25).

---

## 8. Оптимизация размера для роутеров с ограниченным объёмом Flash/RAM

Для маршрутизаторов с малым объёмом постоянной памяти (16–32 МБ Flash) или ограниченной оперативной памятью (32–128 МБ RAM) предусмотрены специальные варианты сборки:

### Варианты сборки и сравнение

| Вариант сборки | Команда | Размер бинарника | Особенности |
|---|---|---|---|
| **Стандартная** | `make build` | ~8.3 МБ | Полный функционал: REST API, Web-интерфейс, метрики Prometheus, тарифы. |
| **Сжатая (UPX)** | `make build-small` | **~2.5 МБ** | Полный функционал, сжатый алгоритмом LZMA/UPX. Экономит до 70% места на Flash. |
| **Без HTTP API (Headless)** | `make build-noapi` | ~7.6 МБ | Отключает HTTP сервер и Web UI (`-tags no_api`). Экономит Flash и снижает потребление RAM. |
| **Headless + UPX** | `make build-noapi && upx --best --lzma bin/gsm2mqtt` | **~2.2 МБ** | Максимально компактная версия для чистого MQTT-шлюза на слабом "железе". |

### Применение в OpenWrt

Если роутер используется исключительно как мост между GSM-модемом и MQTT-брокером (Home Assistant) и веб-интерфейс на роутере не требуется:
1. Соберите бинарник без API:
   ```sh
   make build-noapi
   ```
2. Для дополнительного сжатия под целевую архитектуру (например, `mipsel` или `armv7`):
   ```sh
   upx --best --lzma bin/gsm2mqtt
   ```
3. Скопируйте готовый файл в `/usr/bin/gsm2mqtt` на маршрутизаторе.

