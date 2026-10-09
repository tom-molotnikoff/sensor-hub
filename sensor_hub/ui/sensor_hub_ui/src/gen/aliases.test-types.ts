/**
 * Compile-time assertions for src/gen/aliases.ts.
 * Each entry must resolve to `true`; if it resolves to `never`, tsc fails.
 * Removed alongside the hand-written types when #32 is complete.
 */

import type { components } from './schema';
import type {
  Sensor,
  Reading,
  AggregatedReadingsResponse,
  SensorHealthStatus,
  SensorHealthHistory,
  ConfigFieldSpec,
  DriverInfo,
  MeasurementTypeInfo,
  MQTTBroker,
  MQTTSubscription,
  MQTTBrokerStats,
  AlertRule,
  AlertHistoryEntry,
  Notification,
  UserNotification,
  ChannelPreference,
  Dashboard,
  DashboardConfig,
  DashboardWidget,
  CreateDashboardRequest,
  UpdateDashboardRequest,
  ShareDashboardRequest,
  User,
  RoleInfo,
  PermissionInfo,
  ApiKey,
  EmailSettings,
  LoginResponse,
  MeResponse,
} from './aliases';

declare const _aliases: {
  sensor:                     Sensor                    extends components['schemas']['Sensor']                    ? true : never;
  reading:                    Reading                   extends components['schemas']['Reading']                   ? true : never;
  aggregatedReadingsResponse: AggregatedReadingsResponse extends components['schemas']['AggregatedReadingsResponse'] ? true : never;
  sensorHealthStatus:         SensorHealthStatus        extends components['schemas']['SensorHealthStatus']        ? true : never;
  sensorHealthHistory:        SensorHealthHistory       extends components['schemas']['SensorHealthHistory']       ? true : never;
  configFieldSpec:            ConfigFieldSpec           extends components['schemas']['ConfigFieldSpec']           ? true : never;
  driverInfo:                 DriverInfo                extends components['schemas']['DriverInfo']                ? true : never;
  measurementTypeInfo:        MeasurementTypeInfo       extends components['schemas']['MeasurementType']          ? true : never;
  mqttBroker:                 MQTTBroker                extends components['schemas']['MQTTBroker']                ? true : never;
  mqttSubscription:           MQTTSubscription          extends components['schemas']['MQTTSubscription']          ? true : never;
  mqttBrokerStats:            MQTTBrokerStats           extends components['schemas']['MQTTBrokerStats']           ? true : never;
  alertRule:                  AlertRule                 extends components['schemas']['AlertRule']                 ? true : never;
  alertHistoryEntry:          AlertHistoryEntry         extends components['schemas']['AlertHistoryEntry']         ? true : never;
  notification:               Notification              extends components['schemas']['Notification']              ? true : never;
  userNotification:           UserNotification          extends components['schemas']['UserNotification']          ? true : never;
  channelPreference:          ChannelPreference         extends components['schemas']['ChannelPreference']         ? true : never;
  dashboard:                  Dashboard                 extends components['schemas']['Dashboard']                 ? true : never;
  dashboardConfig:            DashboardConfig           extends components['schemas']['DashboardConfig']           ? true : never;
  dashboardWidget:            DashboardWidget           extends components['schemas']['DashboardWidget']           ? true : never;
  createDashboardRequest:     CreateDashboardRequest    extends components['schemas']['CreateDashboardRequest']    ? true : never;
  updateDashboardRequest:     UpdateDashboardRequest    extends components['schemas']['UpdateDashboardRequest']    ? true : never;
  shareDashboardRequest:      ShareDashboardRequest     extends components['schemas']['ShareDashboardRequest']     ? true : never;
  user:                       User                      extends components['schemas']['User']                      ? true : never;
  roleInfo:                   RoleInfo                  extends components['schemas']['RoleInfo']                  ? true : never;
  permissionInfo:             PermissionInfo            extends components['schemas']['PermissionInfo']            ? true : never;
  apiKey:                     ApiKey                    extends components['schemas']['ApiKey']                    ? true : never;
  emailSettings:              EmailSettings             extends components['schemas']['EmailSettings']             ? true : never;
  loginResponse:              LoginResponse             extends components['schemas']['LoginResponse']             ? true : never;
  meResponse:                 MeResponse                extends components['schemas']['MeResponse']                ? true : never;
};
export type { _aliases };
