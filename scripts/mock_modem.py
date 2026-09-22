#!/usr/bin/env python3
"""
Interactive Virtual GSM Modem Emulator for manual testing of GSM2MQTT.
Simulates a SIMCom SIM800L modem with AT commands, SMS, USSD, and signal quality.
Requires only standard Python 3 libraries.
"""

import os
import pty
import sys
import time

LINK_PATH = "/tmp/ttyGSM0"

def respond(master_fd, text):
    time.sleep(0.05)
    os.write(master_fd, (text + "\r\n").encode("utf-8"))

def main():
    master_fd, slave_fd = pty.openpty()
    slave_name = os.ttyname(slave_fd)

    try:
        if os.path.islink(LINK_PATH) or os.path.exists(LINK_PATH):
            os.remove(LINK_PATH)
        os.symlink(slave_name, LINK_PATH)
        print(f"[Virtual Modem] Created virtual serial port: {slave_name}")
        print(f"[Virtual Modem] Symlinked to: {LINK_PATH}")
        print(f"[Virtual Modem] To use in gsm2mqtt.yaml or .env set: MODEM_DEVICE={LINK_PATH}")
        print("[Virtual Modem] Ready and listening for AT commands (Press Ctrl+C to stop)...")
    except Exception as e:
        print(f"[Virtual Modem] Failed to create symlink: {e}")
        print(f"[Virtual Modem] Use direct slave port: {slave_name}")

    buffer = ""
    in_sms_mode = False

    try:
        while True:
            data = os.read(master_fd, 1024).decode("utf-8", errors="ignore")
            if not data:
                break
            
            buffer += data

            if in_sms_mode:
                # In SMS mode, modem waits for Ctrl+Z (0x1A) or ESC (0x1B)
                if "\x1A" in buffer:
                    print("[Virtual Modem] SMS PDU payload received. Sending +CMGS: 42")
                    respond(master_fd, "\r\n+CMGS: 42\r\n\r\nOK")
                    in_sms_mode = False
                    buffer = ""
                elif "\x1B" in buffer:
                    print("[Virtual Modem] SMS cancelled by client.")
                    respond(master_fd, "\r\nOK")
                    in_sms_mode = False
                    buffer = ""
                continue

            if "\r" in buffer or "\n" in buffer:
                lines = [l.strip() for l in buffer.replace("\r", "\n").split("\n") if l.strip()]
                buffer = ""

                for cmd in lines:
                    cmd_upper = cmd.upper()
                    print(f"[Virtual Modem] << {cmd}")

                    if cmd_upper in ("AT", "ATE0", "ATE1", "AT+CMEE=2", "AT+CLIP=1"):
                        respond(master_fd, "OK")
                    elif cmd_upper == "ATI":
                        respond(master_fd, "SIMCOM_SIM800L\r\nRevision: 1418B04SIM800L24\r\nOK")
                    elif cmd_upper == "AT+CGMI":
                        respond(master_fd, "SIMCOM\r\nOK")
                    elif cmd_upper == "AT+CGMM":
                        respond(master_fd, "SIMCOM_SIM800L\r\nOK")
                    elif cmd_upper == "AT+CPIN?":
                        respond(master_fd, "+CPIN: READY\r\nOK")
                    elif cmd_upper == "AT+CREG?":
                        respond(master_fd, "+CREG: 0,1\r\nOK")
                    elif cmd_upper == "AT+CSQ":
                        respond(master_fd, "+CSQ: 22,0\r\nOK")
                    elif cmd_upper == "AT+COPS?":
                        respond(master_fd, '+COPS: 0,0,"MTS",7\r\nOK')
                    elif cmd_upper.startswith("AT+CMGF="):
                        respond(master_fd, "OK")
                    elif cmd_upper.startswith("AT+CNMI="):
                        respond(master_fd, "OK")
                    elif cmd_upper.startswith("AT+CMGS="):
                        # Modem prompts with '>'
                        print("[Virtual Modem] Prompting for SMS PDU payload ('> ')")
                        os.write(master_fd, b"> ")
                        in_sms_mode = True
                    elif cmd_upper.startswith("AT+CUSD="):
                        respond(master_fd, 'OK\r\n\r\n+CUSD: 0,"Balans: 250.50 rub. Paket: 95 SMS.",15')
                    elif cmd_upper.startswith("ATD"):
                        respond(master_fd, "OK")
                    elif cmd_upper == "ATH":
                        respond(master_fd, "OK")
                    elif cmd_upper.startswith("AT+SAPBR=") or cmd_upper.startswith("AT+HTTP"):
                        respond(master_fd, "OK")
                    else:
                        respond(master_fd, "OK")

    except KeyboardInterrupt:
        print("\n[Virtual Modem] Shutting down...")
    finally:
        os.close(master_fd)
        os.close(slave_fd)
        if os.path.islink(LINK_PATH):
            os.remove(LINK_PATH)
        print("[Virtual Modem] Stopped.")

if __name__ == "__main__":
    main()
