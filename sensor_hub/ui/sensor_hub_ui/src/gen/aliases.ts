/**
 * Convenience type aliases for OpenAPI schema components.
 * Import from here instead of writing `components['schemas']['X']` everywhere.
 */
import type { components } from './schema';

export type Sensor                    = components['schemas']['Sensor'];
export type Reading                   = components['schemas']['Reading'];
export type Capability                = components['schemas']['Capability'];
export type AggregatedReadingsResponse = components['schemas']['AggregatedReadingsResponse'];
export type SensorHealthStatus        = components['schemas']['SensorHealthStatus'];
export type SensorHealthHistory       = components['schemas']['SensorHealthHistory'];
export type ConfigFieldSpec           = components['schemas']['ConfigFieldSpec'];
export type DriverInfo                = components['schemas']['DriverInfo'];
export type MeasurementTypeInfo       = components['schemas']['MeasurementType'];
export type MQTTBroker                = components['schemas']['MQTTBroker'];
export type MQTTSubscription          = components['schemas']['MQTTSubscription'];
export type MQTTBrokerStats           = components['schemas']['MQTTBrokerStats'];
export type AlertRule                 = components['schemas']['AlertRule'];
export type AlertHistoryEntry         = components['schemas']['AlertHistoryEntry'];
export type Notification              = components['schemas']['Notification'];
export type UserNotification          = components['schemas']['UserNotification'];
export type ChannelPreference         = components['schemas']['ChannelPreference'];
export type Dashboard                 = components['schemas']['Dashboard'];
export type DashboardConfig           = components['schemas']['DashboardConfig'];
export type DashboardWidget           = components['schemas']['DashboardWidget'];
export type CreateDashboardRequest    = components['schemas']['CreateDashboardRequest'];
export type UpdateDashboardRequest    = components['schemas']['UpdateDashboardRequest'];
export type ShareDashboardRequest     = components['schemas']['ShareDashboardRequest'];
export type SensorCommandAccepted     = components['schemas']['SensorCommandAccepted'];
export type CommandStatusMessage      = components['schemas']['CommandStatusMessage'];
export type User                      = components['schemas']['User'];
export type RoleInfo                  = components['schemas']['RoleInfo'];
export type PermissionInfo            = components['schemas']['PermissionInfo'];
export type ApiKey                    = components['schemas']['ApiKey'];
export type OAuthStatus               = components['schemas']['OAuthStatus'];
export type LoginResponse             = components['schemas']['LoginResponse'];
export type PropertyDefinition        = components['schemas']['PropertyDefinition'];
export type PropertyGroup             = components['schemas']['PropertyGroup'];
export type PropertyDefinitionsResponse = components['schemas']['PropertyDefinitionsResponse'];
export type MeResponse                = components['schemas']['MeResponse'];
export type TotalReadingsSample       = components['schemas']['TotalReadingsSample'];

export type NotificationSeverity = Notification['severity'];
export type NotificationCategory = Notification['category'];
export type OAuthAuthorizeResponse = components['schemas']['OAuthAuthorizeResponse'];

// Types not in the schema (frontend-only shapes)
export type SensorStatus = 'active' | 'pending' | 'dismissed';
export type ChartEntry = { time: string; [sensor: string]: number | string | null };
