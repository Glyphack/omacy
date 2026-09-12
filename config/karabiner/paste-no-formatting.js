var rule = {
  description: 'omacy: paste with no formatting in chat and browser apps',
  manipulators: [
    {
      type: 'basic',
      from: { key_code: 'v', modifiers: { mandatory: ['command'] } },
      to: [
        {
          key_code: 'v',
          modifiers: ['command', 'shift'],
        },
      ],
      conditions: [
        {
          type: 'frontmost_application_if',
          bundle_identifiers: [
            '.*Slack.*',
            '.*Teams.*',
            '.*Outlook.*',
            '.*Telegram.*',
            '^com\\.apple\\.Notes$',
            '^com\\.google\\.Chrome$',
            '^com\\.brave\\.Browser$',
          ],
        },
      ],
    },
  ],
}

rule
