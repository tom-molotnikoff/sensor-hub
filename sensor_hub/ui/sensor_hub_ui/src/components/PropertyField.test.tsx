import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { PropertyDefinition } from '../gen/aliases';
import PropertyField from './PropertyField';

function makeDefinition(overrides: Partial<PropertyDefinition> = {}): PropertyDefinition {
  return {
    key: 'sensor.discovery.skip',
    label: 'Skip sensor discovery',
    description: "Don't try to auto-discover sensors at startup.",
    type: 'bool',
    default: 'false',
    group: 'sensors',
    apply: 'live',
    readOnly: false,
    ...overrides,
  };
}

describe('PropertyField', () => {
  it('renders a bool property as a switch whose state matches the current value', () => {
    render(<PropertyField definition={makeDefinition()} value="true" onChange={() => {}} />);

    const control = screen.getByRole('switch', { name: 'Skip sensor discovery' });
    expect(control).toBeChecked();
  });

  it('renders an int property as a number field with its unit shown beside the field', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'sensor.collection.interval',
          label: 'Collection interval',
          description: 'How often every enabled sensor is polled.',
          type: 'int',
          unit: 'seconds',
          default: '300',
        })}
        value="300"
        onChange={() => {}}
      />,
    );

    const field = screen.getByRole('spinbutton', { name: 'Collection interval' });
    expect(field).toHaveValue(300);
    expect(screen.getByText('seconds')).toBeInTheDocument();
  });

  it('renders an enum property as a select offering exactly the enum values with the current value selected', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'log.level',
          label: 'Log level',
          description: 'Minimum severity written to the log.',
          type: 'string',
          enum: ['debug', 'info', 'warn', 'error'],
          default: 'info',
        })}
        value="warn"
        onChange={() => {}}
      />,
    );

    const select = screen.getByRole('combobox', { name: 'Log level' });
    expect(select).toHaveTextContent('warn');

    fireEvent.mouseDown(select);
    const options = screen.getAllByRole('option');
    expect(options.map((o) => o.textContent)).toEqual(['debug', 'info', 'warn', 'error']);
  });

  it('renders a string property with no enum as a text field', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'weather.location.name',
          label: 'Location name',
          description: 'Name shown on the weather card.',
          type: 'string',
          default: '',
        })}
        value="Manchester"
        onChange={() => {}}
      />,
    );

    expect(screen.getByRole('textbox', { name: 'Location name' })).toHaveValue('Manchester');
  });

  it('renders a read-only property as plain text with a line explaining why it is fixed', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'database.path',
          label: 'Database file',
          description: 'SQLite database file.',
          type: 'string',
          apply: 'readonly',
          readOnly: true,
          default: 'data/sensor_hub.db',
        })}
        value="/var/lib/sensor-hub/sensor_hub.db"
        onChange={() => {}}
      />,
    );

    expect(screen.getByText('/var/lib/sensor-hub/sensor_hub.db')).toBeInTheDocument();
    expect(screen.getByText(/set at install time/i)).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });

  it('shows the visible label with its description alongside the control', () => {
    render(<PropertyField definition={makeDefinition()} value="false" onChange={() => {}} />);

    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
    expect(screen.getByText("Don't try to auto-discover sensors at startup.")).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeInTheDocument();
  });
});
