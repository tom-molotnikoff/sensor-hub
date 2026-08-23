import { useEffect, useState, useRef } from "react";
import type { Sensor } from "../gen/aliases";
import {WEBSOCKET_BASE} from "../environment/Environment.ts";
import { useAuth } from "../providers/AuthContext.tsx";
import { logger } from '../tools/logger';


function arraysEqual(a: Sensor[], b: Sensor[]) {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    const ai = a[i];
    const bi = b[i];
    if (
      ai.id !== bi.id ||
      ai.name !== bi.name ||
      ai.sensor_driver !== bi.sensor_driver ||
      JSON.stringify(ai.config) !== JSON.stringify(bi.config) ||
      ai.enabled !== bi.enabled ||
      ai.health_status !== bi.health_status ||
      ai.health_reason !== bi.health_reason ||
      ai.retention_hours !== bi.retention_hours
    ) return false;
  }
  return true;
}

export function useSensors() {
  const [sensors, setSensors] = useState<Sensor[]>([]);
  const [loaded, setLoaded] = useState(false);
  const sensorsRef = useRef<Sensor[]>([]);
  const { user } = useAuth();

  // A different user gets a fresh connection, so its list is not loaded yet.
  const [prevUser, setPrevUser] = useState(user);
  if (prevUser !== user) {
    setPrevUser(user);
    setLoaded(false);
  }

  useEffect(() => {
    sensorsRef.current = sensors;
  }, [sensors]);

  useEffect(() => {
    if (user === undefined) return;
    if (user === null) return;

    const ws = new WebSocket(`${WEBSOCKET_BASE}/sensors/ws`);
    ws.onmessage = (event) => {
      try {
        if (!event.data || event.data === "null") {
          setLoaded(true);
          return;
        }
        const parsed = JSON.parse(event.data);
        if (!Array.isArray(parsed)) {
          setLoaded(true);
          return;
        }
        const allSensors = parsed as Sensor[];
        const sortedSensors = allSensors.sort((a, b) => a.name.localeCompare(b.name));

        if (!arraysEqual(sensorsRef.current, sortedSensors)) {
          sensorsRef.current = sortedSensors;
          setSensors(sortedSensors);
        }
        setLoaded(true);
      } catch (err) {
        logger.error("Failed to handle sensors WebSocket message:", err);
      }
    };
    ws.onerror = (err) => {
      logger.error("Sensors WebSocket error:", err);
      setLoaded(true);
    };
    ws.onclose = (event) => {
      logger.debug("Sensors WebSocket closed", event);
      setLoaded(true);
    };
    return () => ws.close();
  }, [user]);

  return { sensors, loaded };
}