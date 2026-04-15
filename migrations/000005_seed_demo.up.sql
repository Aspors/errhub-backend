-- Demo seed data for presentation
-- Login: demo@errhub.local / demo123

-- Fixed UUIDs so down migration can cleanly remove everything
DO $$ BEGIN

  -- User
  INSERT INTO users (id, email, password_hash)
  VALUES (
    '00000000-0000-0000-0000-000000000001',
    'demo@errhub.local',
    '$2a$10$Uxh752M4iAyEKJTDqaXlJ.YJKX/6eelbiXvvqrNf5vcpYLbitodsq'
  ) ON CONFLICT DO NOTHING;

  -- Project
  INSERT INTO projects (id, name, api_key, user_id)
  VALUES (
    '00000000-0000-0000-0000-000000000002',
    'ErrHub Demo App',
    'demo000000000000000000000000000000000000000000000',
    '00000000-0000-0000-0000-000000000001'
  ) ON CONFLICT DO NOTHING;

  -- Issues
  INSERT INTO issues (id, project_id, fingerprint, level, error_type, error_message, occurrences, status, first_seen, last_seen)
  VALUES
    (
      '00000000-0000-0000-0000-000000000010',
      '00000000-0000-0000-0000-000000000002',
      'fp-demo-001',
      'error',
      'TypeError',
      'Cannot read properties of undefined (reading ''user'')',
      47,
      'open',
      NOW() - INTERVAL '5 days',
      NOW() - INTERVAL '2 hours'
    ),
    (
      '00000000-0000-0000-0000-000000000011',
      '00000000-0000-0000-0000-000000000002',
      'fp-demo-002',
      'error',
      'ReferenceError',
      'authToken is not defined',
      12,
      'resolved',
      NOW() - INTERVAL '10 days',
      NOW() - INTERVAL '3 days'
    ),
    (
      '00000000-0000-0000-0000-000000000012',
      '00000000-0000-0000-0000-000000000002',
      'fp-demo-003',
      'warning',
      'NetworkError',
      'Failed to fetch /api/orders: 503 Service Unavailable',
      8,
      'open',
      NOW() - INTERVAL '2 days',
      NOW() - INTERVAL '30 minutes'
    ),
    (
      '00000000-0000-0000-0000-000000000013',
      '00000000-0000-0000-0000-000000000002',
      'fp-demo-004',
      'error',
      'SyntaxError',
      'Unexpected token < in JSON at position 0',
      3,
      'ignored',
      NOW() - INTERVAL '7 days',
      NOW() - INTERVAL '6 days'
    ),
    (
      '00000000-0000-0000-0000-000000000014',
      '00000000-0000-0000-0000-000000000002',
      'fp-demo-005',
      'info',
      'ChunkLoadError',
      'Loading chunk 42 failed after 3 retries',
      2,
      'open',
      NOW() - INTERVAL '1 day',
      NOW() - INTERVAL '1 hour'
    )
  ON CONFLICT DO NOTHING;

  -- Events for issue 010 (TypeError, 5 occurrences spread over last 5 days)
  INSERT INTO events (project_id, issue_id, payload, created_at) VALUES
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000010',
      '{"project_id":"00000000-0000-0000-0000-000000000002","level":"error","release":"demo-v1.0.0","error":{"type":"TypeError","message":"Cannot read properties of undefined (reading ''user'')","stacktrace":"TypeError: Cannot read properties of undefined (reading ''user'')\n    at ProfilePage.setup (src/pages/ProfilePage.vue:42:18)\n    at callWithErrorHandling (node_modules/@vue/runtime-core/dist/runtime-core.esm-bundler.js:7907:24)"},"context":{"url":"https://demo-app.local/profile","user_agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0","framework":"vue"},"breadcrumbs":[{"type":"navigation","message":"Navigated to /profile","timestamp":"2024-01-15T10:29:55Z"},{"type":"click","message":"Clicked #avatar-btn","timestamp":"2024-01-15T10:29:58Z"}]}',
      NOW() - INTERVAL '2 hours'),
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000010',
      '{"project_id":"00000000-0000-0000-0000-000000000002","level":"error","release":"demo-v1.0.0","error":{"type":"TypeError","message":"Cannot read properties of undefined (reading ''user'')","stacktrace":"TypeError: Cannot read properties of undefined (reading ''user'')\n    at ProfilePage.setup (src/pages/ProfilePage.vue:42:18)"},"context":{"url":"https://demo-app.local/profile","user_agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/537.36","framework":"vue"},"breadcrumbs":[{"type":"navigation","message":"Navigated to /profile","timestamp":"2024-01-14T08:10:00Z"}]}',
      NOW() - INTERVAL '1 day'),
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000010',
      '{"project_id":"00000000-0000-0000-0000-000000000002","level":"error","release":"demo-v1.0.0","error":{"type":"TypeError","message":"Cannot read properties of undefined (reading ''user'')","stacktrace":"TypeError: Cannot read properties of undefined (reading ''user'')\n    at ProfilePage.setup (src/pages/ProfilePage.vue:42:18)"},"context":{"url":"https://demo-app.local/profile","user_agent":"Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0","framework":"vue"},"breadcrumbs":[]}',
      NOW() - INTERVAL '3 days');

  -- Events for issue 012 (NetworkError)
  INSERT INTO events (project_id, issue_id, payload, created_at) VALUES
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000012',
      '{"project_id":"00000000-0000-0000-0000-000000000002","level":"warning","release":"demo-v1.0.0","error":{"type":"NetworkError","message":"Failed to fetch /api/orders: 503 Service Unavailable","stacktrace":"NetworkError: Failed to fetch /api/orders: 503 Service Unavailable\n    at OrdersList.fetchOrders (src/modules/Orders/OrdersList.vue:88:12)"},"context":{"url":"https://demo-app.local/orders","user_agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0","framework":"vue"},"breadcrumbs":[{"type":"navigation","message":"Navigated to /orders","timestamp":"2024-01-16T09:00:00Z"},{"type":"request","message":"GET /api/orders → 503","timestamp":"2024-01-16T09:00:01Z"}]}',
      NOW() - INTERVAL '30 minutes');

END $$;
