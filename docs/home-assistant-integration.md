# GSM2MQTT — Спецификация интеграции с Home Assistant

Настоящий документ описывает архитектуру, структуру данных MQTT, спецификацию сущностей Home Assistant (MQTT Auto-Discovery, сенсоры, нотификации) и реализацию интерактивной карточки для дашборда Lovelace.

---

## 1. Архитектура интеграции

```
┌─────────────────┐       Serial (RS232/UART/USB)       ┌────────────────────────┐
│   GSM Modem     │ ◄─────────────────────────────────► │       GSM2MQTT         │
│ (Neoway/Huawei/ │                                     │   (Gateway Service)    │
│  SIMCom/Generic)│                                     │  • PDU SMS / URC / SIM │
└─────────────────┘                                     │  • Call-Drop / USSD    │
                                                        └───────────┬────────────┘
                                                                    │ MQTT (JSON)
                                                                    ▼
                                                        ┌────────────────────────┐
                                                        │  MQTT Broker (Mosquitto)│
                                                        └───────────┬────────────┘
                                                                    │ MQTT / Auto-Discovery
                                                                    ▼
                                                        ┌────────────────────────┐
                                                        │     Home Assistant     │
                                                        │  • Sensors & Binary    │
                                                        │  • notify.gsm_sms      │
                                                        │  • Lovelace Card       │
                                                        └────────────────────────┘
```

---

## 2. Спецификация топиков MQTT и структура данных

По умолчанию префикс топика: `gsm2mqtt`.
Каждый модем идентифицируется параметром `<modem_id>` из конфигурации (например, `neoway_m590`, `modem1`).

### 2.1. Публикуемые топики (Публикация из GSM2MQTT в Home Assistant)

| Топик | QoS | Retain | Описание и формат Payload |
|:---|:---:|:---:|:---|
| `<prefix>/status` | 1 | true | `online` или `offline` (Last Will and Testament, LWT). |
| `<prefix>/gateway/modems` | 1 | true | JSON со списком активных модемов и их статусом: `{"count":1,"active_modem":"neoway","modems":[{"id":"m590","port":"/dev/ttyATH0","status":"ready"}]}`. При отключении модема: `count=0`, `active_modem="none"`, `status="disconnected"`. |
| `<prefix>/modem/<id>/health` | 1 | true | JSON с полным состоянием здоровья модема: `{"status":"ready","signal":20,"signal_dbm":-73,"operator":"MegaFon RUS","network":"GSM","sim":"READY"}`. При физическом отключении модема публикуется со статусом `"status":"disconnected"`, `"sim":"DISCONNECTED"`, обнуляя сигнал и стирая зависший статус `ready`. |
| `<prefix>/modem/<id>/signal` | 1 | false | JSON с уровнем сигнала: `{"rssi":20,"dbm":-73}` |
| `<prefix>/modem/<id>/balance` | 1 | false | Числовое строковое значение текущего баланса: `2.22` |
| `<prefix>/modem/<id>/accounting/status` | 1 | false | JSON статуса тарификации: `{"balance":2.22,"currency":"RUB","modem_id":"neoway_m590","spent_today":0,"sms_sent_today":0}` |
| `<prefix>/modem/<id>/sms/received` | 1 | false | JSON принятого SMS: `{"from":"+79991234567","text":"Hello from GSM","timestamp":"2026-09-23T11:30:00Z","segments":1,"encoding":"GSM-7"}` |
| `<prefix>/modem/<id>/sms/status` | 1 | false | JSON отчета о доставке: `{"ref":12,"recipient":"+79991234567","status":"delivered","code":0,"timestamp":"2026-09-23T11:30:05Z"}` |
| `<prefix>/modem/<id>/call/incoming` | 1 | false | JSON входящего звонка: `{"type":"incoming","from":"+79991234567","modem_id":"neoway_m590"}` |
| `<prefix>/modem/<id>/call/dtmf` | 1 | false | JSON нажатой цифры DTMF: `{"type":"dtmf","digit":"5","modem_id":"neoway_m590"}` |
| `<prefix>/modem/<id>/ussd/response` | 1 | false | JSON ответа на USSD: `{"code":"*100#","response":"Ваш баланс: 2.22 руб.","modem_id":"neoway_m590"}` |
| `<prefix>/modem/<id>/alert` | 1 | false | Текстовое описание аварии (слабый сигнал, ошибка SIM, нет сети). |
| `<prefix>/modem/<id>/event` | 1 | false | JSON структурированных событий (low_balance, quota_warning, quota_exceeded, modem_disconnected, modem_degraded, modem_ready, modem_error, sms_send_failed). Совместим с платформой HA Event. |
| `<prefix>/modem/<id>/command/response` | 1 | false | Текстовый ответ модема на ручную AT-команду. |

### 2.2. Командные топики (Подписка в GSM2MQTT из Home Assistant)

| Топик | Формат Payload | Описание действия |
|:---|:---|:---|
| `<prefix>/modem/<id>/sms/send` | JSON: `{"to":"+79991234567","text":"Alert message","request_report":true}` | Отправка SMS через указанный модем. Поддерживает GSM-7 и кириллицу (UCS-2 или translit). |
| `<prefix>/modem/<id>/call/dial` | JSON: `{"number":"+79991234567"}` или строка: `+79991234567` | Тревожный дозвон Call-Drop. Автоматически сбрасывает звонок при ответе абонента. |
| `<prefix>/modem/<id>/call/hangup` | Любой payload | Принудительное завершение любого активного вызова. |
| `<prefix>/modem/<id>/ussd/send` | Строка кода (например, `*100#`) или JSON `{"code":"*100#"}` | Выполнение произвольного USSD-запроса. |
| `<prefix>/modem/<id>/command/raw` | Строка команды (например, `AT+CSQ`) | Прямая отправка команды модему для диагностики. |

---

## 3. Home Assistant MQTT Auto-Discovery

При включенном параметре `mqtt.discovery: true` GSM2MQTT автоматически регистрирует сущности в Home Assistant.

### 3.1. Структура Device Registry
Все сенсоры модема автоматически группируются под одним устройством в Home Assistant:
- **Identifiers:** `["gsm2mqtt_<modem_id>"]`
- **Name:** `GSM Modem (<modem_id>)`
- **Manufacturer:** Производитель чипсета (`Neoway`, `Huawei`, `SIMCom` и др.)
- **Model:** Модель модема (`M590`, `E1550`, `SIM800` и др.)
- **Via Device:** `gsm2mqtt_gateway`

### 3.2. Автоматически создаваемые сущности

1. **Уровень сигнала (dBm):**
   - **Entity ID:** `sensor.<modem_id>_signal`
   - **Device Class:** `signal_strength`
   - **Unit:** `dBm`
   - **State Topic:** `gsm2mqtt/modem/<id>/signal`
   - **Value Template:** `{{ value_json.dbm }}`

2. **Статус модема:**
   - **Entity ID:** `sensor.<modem_id>_status`
   - **Device Class:** `enum`
   - **Options:** `["ready", "degraded", "not_ready", "error", "disconnected"]`
   - **State Topic:** `gsm2mqtt/modem/<id>/health`
   - **Value Template:** `{{ value_json.status }}`
   - **Значения:**
     - `ready`: Модем инициализирован, зарегистрирован в сети и полностью готов к работе.
     - `degraded`: Модем на связи, но уровень сигнала слабый (`CSQ < 5`) либо нет регистрации в сети.
     - `not_ready`: SIM-карта не готова (требуется PIN/PUK, либо заблокирована).
     - `error`: Аппаратный сбой или ошибка исполнения AT-команд.
     - `disconnected`: Модем физически отключен, серийный порт недоступен или питание снято (карточка Home Assistant четко показывает отключение модема, а не зависает в `ready`).

3. **Баланс SIM-карты:**
   - **Entity ID:** `sensor.<modem_id>_balance`
   - **Device Class:** `monetary`
   - **Unit:** `RUB`
   - **State Topic:** `gsm2mqtt/modem/<id>/balance`

4. **Физическое подключение модема:**
   - **Entity ID:** `binary_sensor.<modem_id>_connected`
   - **Device Class:** `connectivity`
   - **Icon:** `mdi:connection`
   - **State Topic:** `gsm2mqtt/modem/<id>/health`
   - **Value Template:** `{{ 'OFF' if value_json.status == 'disconnected' else 'ON' }}`

5. **Аппаратная проблема модема:**
   - **Entity ID:** `binary_sensor.<modem_id>_problem`
   - **Device Class:** `problem`
   - **Icon:** `mdi:alert-circle-outline`
   - **State Topic:** `gsm2mqtt/modem/<id>/health`
   - **Value Template:** `{{ 'ON' if value_json.status in ['error', 'not_ready', 'disconnected'] else 'OFF' }}`

6. **Предупреждение о низком балансе:**
   - **Entity ID:** `binary_sensor.<modem_id>_low_balance`
   - **Device Class:** `problem`
   - **Icon:** `mdi:cash-alert`
   - **State Topic:** `gsm2mqtt/modem/<id>/accounting/status`
   - **Value Template:** `{{ 'ON' if value_json.low_balance else 'OFF' }}`

7. **Поток событий (Home Assistant Event Entity):**
   - **Entity ID:** `event.<modem_id>_events`
   - **Icon:** `mdi:bell-badge-outline`
   - **State Topic:** `gsm2mqtt/modem/<id>/event`
   - **Event Types:** `low_balance`, `sms_limit_warning`, `sms_limit_exceeded`, `call_minutes_warning`, `call_minutes_exceeded`, `data_limit_warning`, `data_limit_exceeded`, `modem_disconnected`, `modem_degraded`, `modem_ready`, `modem_error`, `sms_send_failed`.

---

## 4. Ручная настройка сущностей в `configuration.yaml`

Если автодискавери выключен или требуются дополнительные кастомные сенсоры, добавьте следующую конфигурацию в `configuration.yaml` Home Assistant:

```yaml
mqtt:
  sensor:
    # Баланс SIM-карты
    - name: "GSM Modem Balance"
      unique_id: "gsm_modem_balance"
      state_topic: "gsm2mqtt/modem/neoway_m590/balance"
      unit_of_measurement: "RUB"
      device_class: "monetary"
      icon: "mdi:cash"

    # Уровень сигнала
    - name: "GSM Signal Strength"
      unique_id: "gsm_modem_signal"
      state_topic: "gsm2mqtt/modem/neoway_m590/signal"
      unit_of_measurement: "dBm"
      value_template: "{{ value_json.dbm }}"
      device_class: "signal_strength"

    # Оператор связи
    - name: "GSM Operator"
      unique_id: "gsm_modem_operator"
      state_topic: "gsm2mqtt/modem/neoway_m590/health"
      value_template: "{{ value_json.operator }}"
      icon: "mdi:cellphone-tower"

    # Последнее принятое SMS
    - name: "GSM Last Incoming SMS"
      unique_id: "gsm_modem_last_sms"
      state_topic: "gsm2mqtt/modem/neoway_m590/sms/received"
      value_template: "{{ value_json.text }}"
      json_attributes_topic: "gsm2mqtt/modem/neoway_m590/sms/received"
      icon: "mdi:message-text"

  binary_sensor:
    # Входящий звонок
    - name: "GSM Incoming Call"
      unique_id: "gsm_incoming_call"
      state_topic: "gsm2mqtt/modem/neoway_m590/call/incoming"
      payload_on: "incoming"
      value_template: "{{ value_json.type }}"
      off_delay: 30
      icon: "mdi:phone-ring"
```

---

## 5. Объект отправки уведомлений (`notify.gsm_sms`)

Для отправки SMS уведомлений из стандартных автоматизаций Home Assistant создается скрипт или сервис `notify`.

### Способ 1: Скрипт-нотификатор (Рекомендуется)

Добавьте в `scripts.yaml`:

```yaml
send_gsm_sms:
  alias: "Отправить SMS через GSM2MQTT"
  icon: "mdi:message-processing"
  fields:
    target:
      description: "Номер получателя в формате +79XXXXXXXXX"
      example: "+79001234567"
      required: true
    message:
      description: "Текст SMS сообщения"
      example: "Внимание! Сработал датчик протечки в ванной!"
      required: true
  sequence:
    - service: mqtt.publish
      data:
        topic: "gsm2mqtt/modem/neoway_m590/sms/send"
        payload: >
          {
            "to": "{{ target }}",
            "text": "{{ message }}",
            "request_report": true
          }

trigger_call_drop:
  alias: "Тревожный дозвон Call-Drop"
  icon: "mdi:phone-alert"
  fields:
    number:
      description: "Номер телефона для дозвона"
      example: "+79001234567"
      required: true
  sequence:
    - service: mqtt.publish
      data:
        topic: "gsm2mqtt/modem/neoway_m590/call/dial"
        payload: >
          {"number": "{{ number }}"}
```

### Способ 2: Интеграция с `notify` платформой

Добавьте в `configuration.yaml`:

```yaml
notify:
  - name: gsm_sms
    platform: rest
    resource: "http://127.0.0.1:8088/api/sms/send"
    method: "POST_JSON"
    data:
      modem_id: "neoway_m590"
    title_param_name: "to"
    message_param_name: "text"
```

### Пример использования в автоматизации:

```yaml
automation:
  - alias: "Оповещение о протечке воды"
    trigger:
      - platform: state
        entity_id: binary_sensor.water_leak_bathroom
        to: "on"
    action:
      # 1. Отправляем тревожное SMS
      - service: script.send_gsm_sms
        data:
          target: "+79964126670"
          message: "ТРЕВОГА: Протечка воды в ванной! Вода перекрыта автоматически."
      # 2. Выполняем тревожный дозвон-сброс (Call-Drop)
      - service: script.trigger_call_drop
        data:
          number: "+79964126670"
```

---

## 6. Спецификация Lovelace карточки (`gsm2mqtt-card`)

Интерактивная карточка для дашборда Home Assistant объединяет в себе все функции управления модемом.

### 6.1. Макет карточки

```
┌────────────────────────────────────────────────────────┐
│  📟 GSM Модем (Neoway M590)              [ READY ]     │
├────────────────────────────────────────────────────────┤
│  Оператор: MegaFon RUS    │  Сигнал: ▇▇▇▇  -73 dBm (20)│
│  Баланс:   2.22 RUB       │  MQTT:   Connected         │
├────────────────────────────────────────────────────────┤
│  ⚡ USSD Запрос                                         │
│  [ *100#          ]  [ Отправить ]                     │
│  Быстрые: [*100# Баланс] [*105# Тариф]                 │
├────────────────────────────────────────────────────────┤
│  ✉️ Отправка SMS                                       │
│  Кому:  [ +79001234567         ]                       │
│  Текст: [ Тревога: сработал датчик...                ] │
│         Символов: 28 (1 SMS, GSM-7)                    │
│  [ Отправить SMS ]                                     │
├────────────────────────────────────────────────────────┤
│  📞 Звонок-сброс (Call-Drop)                           │
│  [ +79001234567         ]  [ Позвонить ] [ Сброс ]     │
├────────────────────────────────────────────────────────┤
│  📥 Входящие SMS (Последние 5 сообщений)              │
│  • 11:21 +79964126670: "Ваш баланс: 2.22 руб."        │
│  • 09:15 MegaFon: "Вам поступил платеж"               │
└────────────────────────────────────────────────────────┘
```

### 6.2. Исходный код карточки (`gsm2mqtt-card.js`)

Сохраните следующий файл в папку Home Assistant: `config/www/gsm2mqtt-card.js`
и добавьте ресурс в Lovelace (`/local/gsm2mqtt-card.js?v=1.0.0` как JavaScript Module).

```javascript
class GSM2MQTTCard extends HTMLElement {
  set hass(hass) {
    this._hass = hass;
    if (!this.content) {
      this.initCard();
    }
    this.updateCard();
  }

  setConfig(config) {
    this._config = Object.assign({
      modem_id: 'neoway_m590',
      topic_prefix: 'gsm2mqtt',
      title: 'GSM Gateway'
    }, config);
  }

  initCard() {
    this.innerHTML = `
      <ha-card header="${this._config.title}">
        <style>
          .card-content { padding: 0 16px 16px; font-family: var(--paper-font-body1_-_font-family); }
          .status-row { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px; }
          .metric-box { background: var(--secondary-background-color); padding: 8px 12px; border-radius: 8px; }
          .metric-val { font-size: 1.15em; font-weight: bold; margin-top: 4px; }
          .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.8em; font-weight: bold; }
          .badge-ready { background: #e8f5e9; color: #2e7d32; }
          .badge-disconnected { background: #efebe9; color: #5d4037; }
          .badge-error { background: #ffebee; color: #c62828; }
          .input-row { display: flex; gap: 8px; margin-top: 8px; }
          input, textarea { width: 100%; padding: 8px; border: 1px solid var(--divider-color); border-radius: 6px; background: var(--card-background-color); color: var(--primary-text-color); }
          button { padding: 8px 16px; background: var(--primary-color); color: #fff; border: none; border-radius: 6px; cursor: pointer; font-weight: 500; }
          button:disabled { opacity: 0.6; cursor: not-allowed; }
          .section-title { font-weight: 600; font-size: 0.95em; margin: 12px 0 4px; display: flex; align-items: center; gap: 6px; }
          .quick-btn { padding: 3px 8px; font-size: 0.75em; background: var(--secondary-background-color); color: var(--primary-text-color); border: 1px solid var(--divider-color); border-radius: 4px; cursor: pointer; }
          .sms-table { width: 100%; border-collapse: collapse; font-size: 0.85em; margin-top: 8px; }
          .sms-table th, .sms-table td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--divider-color); }
        </style>
        <div class="card-content">
          <div class="status-row">
            <div class="metric-box">
              <div style="font-size:0.8em; color:var(--secondary-text-color)">Статус & Баланс</div>
              <div class="metric-val" id="valBalance">...</div>
              <div id="valStatus" style="margin-top:4px;"><span class="badge badge-ready">READY</span></div>
            </div>
            <div class="metric-box">
              <div style="font-size:0.8em; color:var(--secondary-text-color)">Сигнал & Оператор</div>
              <div class="metric-val" id="valSignal">...</div>
              <div id="valOperator" style="font-size:0.85em; margin-top:4px; color:var(--secondary-text-color)">-</div>
            </div>
          </div>

          <!-- USSD -->
          <div class="section-title">⚡ USSD Запрос</div>
          <div class="input-row">
            <input type="text" id="ussdInput" value="*100#">
            <button id="ussdBtn">Запрос</button>
          </div>
          <div style="display:flex; gap:6px; margin-top:4px;">
            <span class="quick-btn" onclick="document.getElementById('ussdInput').value='*100#'">*100# Баланс</span>
            <span class="quick-btn" onclick="document.getElementById('ussdInput').value='*105#'">*105# Тариф</span>
          </div>

          <!-- Send SMS -->
          <div class="section-title">✉️ Отправить SMS</div>
          <input type="text" id="smsTo" placeholder="+79001234567" style="margin-bottom:6px;">
          <textarea id="smsText" rows="2" placeholder="Текст сообщения..."></textarea>
          <div class="input-row" style="justify-content:flex-end;">
            <button id="smsBtn">Отправить</button>
          </div>

          <!-- Voice Call (Call-Drop) -->
          <div class="section-title">📞 Звонок-Сброс (Call-Drop)</div>
          <div class="input-row">
            <input type="text" id="callNumber" placeholder="+79001234567">
            <button id="callBtn" style="background:#2e7d32">Вызов</button>
            <button id="hangupBtn" style="background:#c62828">Сброс</button>
          </div>

          <!-- Inbox -->
          <div class="section-title">📥 Последнее сообщение</div>
          <div id="lastSMSBox" style="font-size:0.85em; padding:8px; background:var(--secondary-background-color); border-radius:6px; min-height:36px;">
            Нет входящих сообщений
          </div>
        </div>
      </ha-card>
    `;
    this.content = this.querySelector('.card-content');
    this.bindEvents();
  }

  bindEvents() {
    const pub = (subTopic, payload) => {
      const topic = `${this._config.topic_prefix}/modem/${this._config.modem_id}/${subTopic}`;
      this._hass.callService('mqtt', 'publish', { topic, payload });
    };

    this.querySelector('#ussdBtn').onclick = () => {
      const code = this.querySelector('#ussdInput').value;
      if (code) pub('ussd/send', code);
    };

    this.querySelector('#smsBtn').onclick = () => {
      const to = this.querySelector('#smsTo').value;
      const text = this.querySelector('#smsText').value;
      if (to && text) {
        pub('sms/send', JSON.stringify({ to, text, request_report: true }));
        this.querySelector('#smsText').value = '';
      }
    };

    this.querySelector('#callBtn').onclick = () => {
      const num = this.querySelector('#callNumber').value;
      if (num) pub('call/dial', JSON.stringify({ number: num }));
    };

    this.querySelector('#hangupBtn').onclick = () => {
      pub('call/hangup', '{}');
    };
  }

  updateCard() {
    if (!this._hass) return;
    const p = this._config.topic_prefix;
    const m = this._config.modem_id;

    // Чтение сущностей из Home Assistant states
    const balState = this._hass.states[`sensor.${m}_balance`];
    if (balState) {
      this.querySelector('#valBalance').innerText = `${balState.state} ${balState.attributes.unit_of_measurement || 'RUB'}`;
    }

    const sigState = this._hass.states[`sensor.${m}_signal`];
    if (sigState) {
      this.querySelector('#valSignal').innerText = `${sigState.state} dBm`;
    }

    const opState = this._hass.states[`sensor.${m}_operator`];
    if (opState) {
      this.querySelector('#valOperator').innerText = opState.state;
    }

    const statusState = this._hass.states[`sensor.${m}_status`];
    if (statusState) {
      const st = statusState.state;
      const isReady = st === 'ready';
      const isDisc = st === 'disconnected';
      const cls = isReady ? 'badge-ready' : isDisc ? 'badge-disconnected' : 'badge-error';
      const label = isReady ? 'READY' : isDisc ? 'DISCONNECTED' : st.toUpperCase();
      this.querySelector('#valStatus').innerHTML = `<span class="badge ${cls}">${label}</span>`;
    }

    const smsState = this._hass.states[`sensor.${m}_last_sms`];
    if (smsState && smsState.state !== 'unknown') {
      const from = smsState.attributes.from || 'Отправитель';
      this.querySelector('#lastSMSBox').innerHTML = `<strong>${from}:</strong> ${smsState.state}`;
    }
  }

  getCardSize() {
    return 6;
  }
}

customElements.define('gsm2mqtt-card', GSM2MQTTCard);
```

---

## 7. Спецификация Home Assistant Add-on (`Hass.io`)

Для распространения GSM2MQTT как официального или кастомного дополнения Home Assistant создается репозиторий со следующей файловой структурой:

### 7.1. Файловая структура репозитория аддонов
```
ha-addons-gsm2mqtt/
├── repository.yaml
└── gsm2mqtt/
    ├── config.yaml
    ├── Dockerfile
    ├── build.yaml
    ├── run.sh
    └── DOCS.md
```

### 7.2. `gsm2mqtt/config.yaml`
```yaml
name: "GSM2MQTT Gateway"
description: "Управление GSM-модемами через MQTT: SMS, звонки-сбросы, USSD и Home Assistant Dashboard"
version: "1.0.0"
slug: "gsm2mqtt"
init: false
arch:
  - aarch64
  - amd64
  - armv7
startup: services
boot: auto
uart: true
usb: true
ports:
  8080/tcp: 8088
ports_description:
  8080/tcp: "Встроенная веб-панель управления GSM2MQTT"
options:
  serial_port: "/dev/ttyUSB0"
  baud_rate: 9600
  modem_type: "neoway"
  modem_id: "modem1"
  mqtt_broker: "core-mosquitto"
  mqtt_port: 1883
  mqtt_topic_prefix: "gsm2mqtt"
schema:
  serial_port: "device(subsystem=tty)"
  baud_rate: "int"
  modem_type: "list(generic|neoway|huawei|simcom|siemens|cinterion)"
  modem_id: "str"
  mqtt_broker: "str"
  mqtt_port: "int"
  mqtt_topic_prefix: "str"
```

### 7.3. `gsm2mqtt/run.sh`
```bash
#!/usr/bin/with-contenv bashio

SERIAL_PORT=$(bashio::config 'serial_port')
BAUD_RATE=$(bashio::config 'baud_rate')
MODEM_TYPE=$(bashio::config 'modem_type')
MODEM_ID=$(bashio::config 'modem_id')
MQTT_BROKER=$(bashio::config 'mqtt_broker')
MQTT_PORT=$(bashio::config 'mqtt_port')
TOPIC_PREFIX=$(bashio::config 'mqtt_topic_prefix')

bashio::log.info "Запуск GSM2MQTT Gateway на порту ${SERIAL_PORT} (${MODEM_TYPE}, ${BAUD_RATE} baud)..."

cat <<EOF > /etc/gsm2mqtt/gsm2mqtt.yaml
mqtt:
  broker: "${MQTT_BROKER}"
  port: ${MQTT_PORT}
  topic_prefix: "${TOPIC_PREFIX}"
  discovery: true
  discovery_prefix: "homeassistant"
api:
  enabled: true
  host: "0.0.0.0"
  port: 8080
modems:
  - id: "${MODEM_ID}"
    type: "${MODEM_TYPE}"
    port: "${SERIAL_PORT}"
    baud_rate: ${BAUD_RATE}
EOF

exec /app/gsm2mqtt
```
