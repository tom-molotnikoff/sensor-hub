import {BrowserRouter, Route, Routes, Navigate} from "react-router";
import {Suspense, lazy} from "react";
import SensorsOverview from "../pages/sensors-overview/SensorsOverview.tsx";
import {useSensorContext} from "../hooks/useSensorContext.ts";
import SensorPage from "../pages/sensor/SensorPage.tsx";
import PropertiesOverview from "../pages/properties/PropertiesOverview.tsx";
import LoginPage from "../pages/Login.tsx";
import ChangePasswordPage from "../pages/account/ChangePassword.tsx";
import SessionsPage from "../pages/account/SessionsPage.tsx";
import UsersPage from "../pages/admin/UsersPage.tsx";
import NotificationsPage from "../pages/notifications/NotificationsPage.tsx";
import RequireAuth from "./RequireAuth.tsx";
import DashboardPage from "../dashboard/DashboardPage.tsx";
import MqttPage from "../pages/mqtt/MqttPage.tsx";
import DataRetentionPage from "../pages/data-retention/DataRetentionPage.tsx";

const DeveloperPage = lazy(() => import("../pages/account/DeveloperPage.tsx"));


function AppRoutes() {
  const {sensors} = useSensorContext();

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/account/change-password" element={<RequireAuth><ChangePasswordPage /></RequireAuth>} />
        <Route path="/account/sessions" element={<RequireAuth><SessionsPage /></RequireAuth>} />
        <Route path="/account/developer" element={<RequireAuth><Suspense fallback={null}><DeveloperPage /></Suspense></RequireAuth>} />
        <Route path="/admin" element={<RequireAuth><UsersPage /></RequireAuth>} />
        <Route path="/mqtt" element={<RequireAuth><MqttPage /></RequireAuth>} />
        <Route path="/notifications" element={<RequireAuth><NotificationsPage /></RequireAuth>} />
        <Route path="/dashboard" element={<RequireAuth><DashboardPage /></RequireAuth>} />
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/sensors-overview" element={<RequireAuth><SensorsOverview /></RequireAuth>} />
        { sensors.map((sensor) => {
          return (
            <Route
              key={sensor.id}
              path={`/sensor/${sensor.id}`}
              element={<RequireAuth><SensorPage sensorId={sensor.id} /></RequireAuth>}
            />
          )
        })}
        <Route path="/properties-overview" element={<RequireAuth><PropertiesOverview /></RequireAuth>} />
        <Route path="/data-retention" element={<RequireAuth><DataRetentionPage /></RequireAuth>} />
      </Routes>
    </BrowserRouter>
  )
}

export default AppRoutes