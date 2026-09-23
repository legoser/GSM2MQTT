// Package drivers provides GSM modem hardware drivers implementing the
// modem.Driver interface across diverse modem chipsets and manufacturers.
//
// Supported hardware drivers:
//
//   - GenericDriver: Baseline 3GPP TS 27.005 / 27.007 implementation suitable
//     for standard AT-compliant modems.
//
//   - NeowayDriver: Dedicated driver for Neoway M590 and M590E GSM/GPRS modules.
//     Includes autobaud sync detection, IRA character set enforcement, and
//     call state polling via AT+CLCC for Call-Drop voice alerts.
//
//   - HuaweiDriver: Driver for Huawei USB modems (E1550, E173, etc.) with CNMI
//     notification fallback chains and mode switching support.
//
//   - SIMComDriver: Driver for SIMCom SIM800 and SIM900 series cellular modules.
//
//   - SiemensDriver: Driver for Siemens TC35 / MC35 industrial cellular terminals.
//
//   - CinterionDriver: Driver for Cinterion BGS2 and MC55i series modules.
package drivers
