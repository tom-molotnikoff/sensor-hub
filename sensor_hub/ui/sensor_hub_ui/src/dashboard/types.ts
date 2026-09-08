import type { ComponentType } from 'react';
import type { WidgetKind } from './WidgetContext';

export interface WidgetProps {
    id: string;
    config: Record<string, unknown>;
    isEditing: boolean;
}

export interface WidgetDefinition {
    type: string;
    label: string;
    description: string;
    kind: WidgetKind;
    component: ComponentType<WidgetProps>;
    defaultConfig: Record<string, unknown>;
    defaultLayout: { w: number; h: number };
    minW?: number;
    minH?: number;
    maxW?: number;
    maxH?: number;
    configFields?: WidgetConfigField[];
}

export interface WidgetConfigField {
    key: string;
    label: string;
    type: 'text' | 'textarea' | 'number' | 'boolean' | 'select' | 'sensor-select' | 'controllable-sensor-select' | 'binary-capability-select' | 'multi-sensor-select' | 'date' | 'measurement-type-select' | 'time-range' | 'aggregation-function-select';
    options?: { value: string; label: string }[];
    defaultValue?: unknown;
}
