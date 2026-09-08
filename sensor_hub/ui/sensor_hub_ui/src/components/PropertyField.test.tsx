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
  it('reflects the default in the control of a definition carrying no value', () => {
    render(<PropertyField definition={makeDefinition({ default: 'true' })} onChange={() => {}} />);

    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeChecked();
  });

  it('states only the saved value for an undescribed property, with no default, chip or consequence note', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'mqtt.broker.enabled',
          label: 'mqtt.broker.enabled',
          description: '',
          type: 'string',
          default: '',
          apply: 'live',
        })}
        described={false}
        serverValue="true"
        editedValue="false"
        onChange={() => {}}
      />,
    );

    expect(screen.getByText('Saved value true')).toBeInTheDocument();
    expect(screen.queryByText(/default/)).not.toBeInTheDocument();
    expect(screen.queryByText(/disconnects connected sensors/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /restart/i })).not.toBeInTheDocument();
    expect(screen.getAllByText('mqtt.broker.enabled')).toHaveLength(1);
  });

  it('renders a bool property as a switch whose state matches the current value', () => {
    render(<PropertyField definition={makeDefinition()} serverValue="true" onChange={() => {}} />);

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
        serverValue="300"
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
        serverValue="warn"
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
        serverValue="Manchester"
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
        serverValue="/var/lib/sensor-hub/sensor_hub.db"
        onChange={() => {}}
      />,
    );

    expect(screen.getByText('/var/lib/sensor-hub/sensor_hub.db')).toBeInTheDocument();
    expect(screen.getByText(/set at install time/i)).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });

  it('shows a helper line with saved value and default only once the property is modified', () => {
    const definition = makeDefinition({
      key: 'auth.bcrypt.cost',
      label: 'Bcrypt cost',
      description: 'Work factor for password hashing.',
      type: 'int',
      default: '12',
      group: 'security',
      apply: 'live',
    });

    const { rerender } = render(
      <PropertyField definition={definition} serverValue="12" onChange={() => {}} />,
    );
    expect(screen.queryByText(/saved value/i)).not.toBeInTheDocument();

    rerender(<PropertyField definition={definition} serverValue="12" editedValue="14" onChange={() => {}} />);
    expect(screen.getByRole('spinbutton', { name: 'Bcrypt cost' })).toHaveValue(14);
    expect(screen.getByText('Saved value 12 · default 12')).toBeInTheDocument();

    // An edit typed back to the saved value leaves the row unmodified.
    rerender(<PropertyField definition={definition} serverValue="12" editedValue="12" onChange={() => {}} />);
    expect(screen.queryByText(/saved value/i)).not.toBeInTheDocument();
  });

  it('states in the helper line that a modified next-cycle property applies from the next cycle, naming the cycle length', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'sensor.collection.interval',
          label: 'Collection interval',
          description: 'How often every enabled sensor is polled.',
          type: 'int',
          unit: 'seconds',
          default: '300',
          apply: 'next-cycle',
        })}
        serverValue="300"
        editedValue="120"
        onChange={() => {}}
      />,
    );

    expect(
      screen.getByText('Saved value 300 · default 300 · applies from the next cycle (currently 300 seconds)'),
    ).toBeInTheDocument();
  });

  it('shows one non-interactive chip naming what an action property requires, and none for a live property', () => {
    const { rerender } = render(
      <PropertyField
        definition={makeDefinition({
          key: 'mqtt.broker.port',
          label: 'Broker port',
          description: 'TCP port the embedded broker listens on.',
          type: 'int',
          default: '1883',
          group: 'mqtt',
          apply: 'action:service-restart',
        })}
        serverValue="1883"
        onChange={() => {}}
      />,
    );
    const restartChip = screen.getByText('Service restart required');
    expect(restartChip).toBeInTheDocument();
    expect(restartChip.closest('button')).toBeNull();

    rerender(
      <PropertyField
        definition={makeDefinition({
          key: 'oauth.credentials.file.path',
          label: 'OAuth credentials file',
          description: 'Path to the OAuth client credentials file.',
          type: 'string',
          default: '',
          group: 'email',
          apply: 'action:oauth-reload',
        })}
        serverValue="/etc/sensor-hub/credentials.json"
        onChange={() => {}}
      />,
    );
    expect(screen.getByText('OAuth reload required')).toBeInTheDocument();
    expect(screen.queryByText('Service restart required')).not.toBeInTheDocument();

    rerender(<PropertyField definition={makeDefinition()} serverValue="false" onChange={() => {}} />);
    expect(screen.queryByText(/required/)).not.toBeInTheDocument();
  });

  it('carries a muted consequence line on the MQTT properties and a Notifications pointer on the OAuth file paths', () => {
    const { rerender } = render(
      <PropertyField
        definition={makeDefinition({
          key: 'mqtt.broker.enabled',
          label: 'Broker enabled',
          description: 'Whether the embedded MQTT broker runs.',
          type: 'bool',
          default: 'true',
          group: 'mqtt',
          apply: 'action:service-restart',
        })}
        serverValue="true"
        onChange={() => {}}
      />,
    );
    expect(screen.getByText('Changing this disconnects connected sensors.')).toBeInTheDocument();

    rerender(
      <PropertyField
        definition={makeDefinition({
          key: 'oauth.token.file.path',
          label: 'OAuth token file',
          description: 'Path to the stored OAuth token.',
          type: 'string',
          default: '',
          group: 'email',
          apply: 'action:oauth-reload',
        })}
        serverValue="/etc/sensor-hub/token.json"
        onChange={() => {}}
      />,
    );
    expect(screen.queryByText('Changing this disconnects connected sensors.')).not.toBeInTheDocument();
    expect(screen.getByText(/reload config on the notifications page/i)).toBeInTheDocument();
  });

  it('renders an empty saved value or default as "(empty)" in the helper line', () => {
    const definition = makeDefinition({
      key: 'smtp.user',
      label: 'SMTP user',
      description: 'Address mail is sent from.',
      type: 'string',
      default: '',
      group: 'email',
      apply: 'live',
    });

    const { rerender } = render(
      <PropertyField definition={definition} serverValue="old@example.com" editedValue="new@example.com" onChange={() => {}} />,
    );
    expect(screen.getByText('Saved value old@example.com · default (empty)')).toBeInTheDocument();

    // Edited before the value feed has delivered anything.
    rerender(<PropertyField definition={definition} editedValue="new@example.com" onChange={() => {}} />);
    expect(screen.getByText('Saved value (empty) · default (empty)')).toBeInTheDocument();
  });

  it('singularizes the cycle length unit when the saved interval is 1', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'data.cleanup.interval.hours',
          label: 'Cleanup interval',
          description: 'How often old data is cleaned up.',
          type: 'int',
          unit: 'hours',
          default: '1',
          apply: 'next-cycle',
        })}
        serverValue="1"
        editedValue="2"
        onChange={() => {}}
      />,
    );

    expect(
      screen.getByText('Saved value 1 · default 1 · applies from the next cycle (currently 1 hour)'),
    ).toBeInTheDocument();
  });

  it('falls back to the raw action id so an unrecognised action never renders as live', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'mqtt.broker.port',
          label: 'Broker port',
          description: 'TCP port the embedded broker listens on.',
          type: 'int',
          default: '1883',
          group: 'mqtt',
          apply: 'action:broker-bounce',
        })}
        serverValue="1883"
        editedValue="8883"
        onChange={() => {}}
      />,
    );

    expect(screen.getByText('Requires: broker-bounce')).toBeInTheDocument();
    expect(
      screen.getByText('Saved value 1883 · default 1883 · applies after broker-bounce'),
    ).toBeInTheDocument();
  });

  it('carries the required action as the apply state in a modified action property helper line', () => {
    render(
      <PropertyField
        definition={makeDefinition({
          key: 'mqtt.broker.port',
          label: 'Broker port',
          description: 'TCP port the embedded broker listens on.',
          type: 'int',
          default: '1883',
          group: 'mqtt',
          apply: 'action:service-restart',
        })}
        serverValue="1883"
        editedValue="8883"
        onChange={() => {}}
      />,
    );

    expect(
      screen.getByText('Saved value 1883 · default 1883 · applies after a service restart'),
    ).toBeInTheDocument();
  });

  it('shows the raw key alongside the label', () => {
    render(<PropertyField definition={makeDefinition()} serverValue="false" onChange={() => {}} />);

    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
    expect(screen.getByText('sensor.discovery.skip')).toBeInTheDocument();
  });

  it('shows the visible label with its description alongside the control', () => {
    render(<PropertyField definition={makeDefinition()} serverValue="false" onChange={() => {}} />);

    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
    expect(screen.getByText("Don't try to auto-discover sensors at startup.")).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeInTheDocument();
  });
});
