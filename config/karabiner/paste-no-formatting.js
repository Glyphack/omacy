var rule = {
  description: 'omacy: cmd shift v pastes with no formatting',
  manipulators: [
    {
      type: 'basic',
      from: { key_code: 'v', modifiers: { mandatory: ['command', 'shift'] } },
      to: [
        {
          key_code: 'v',
          modifiers: ['command', 'shift', 'option'],
        },
      ],
      conditions: [
        {
          type: 'frontmost_application_if',
          bundle_identifiers: [
            '^com\\.microsoft\\.Outlook$',
            '^com\\.apple\\.Notes$',
          ],
        },
      ],
    },
  ],
}

rule
