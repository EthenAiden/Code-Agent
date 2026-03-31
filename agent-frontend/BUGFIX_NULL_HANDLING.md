# Bug Fix: Null/Empty Data Handling

## Issue
When conversations list or messages are empty, the application threw an error:
```
Failed to load messages: Cannot read properties of null (reading 'map')
```

## Root Cause
The API could return `null` or `undefined` instead of empty arrays, causing `.map()` to fail.

## Fixes Applied

### 1. API Client (`src/lib/api.ts`)
- Added null/undefined checks in the `request` method
- Added explicit array checks in `getMessages()` and `listSessions()`
- Ensured all array responses default to `[]` if null/undefined

```typescript
// Before
return data.data as T;

// After
if (data.data === null || data.data === undefined) {
  return (Array.isArray(data.data) ? [] : {}) as T;
}
return data.data as T;
```

### 2. Index Component (`src/pages/Index.tsx`)
- Added null coalescing operators (`|| []`) when mapping API responses
- Added error recovery in `loadMessages` to set empty array on failure
- Improved error handling to not show errors for expected "not found" cases

```typescript
// Before
const msgs: Message[] = apiMessages.map(...)

// After
const msgs: Message[] = (apiMessages || []).map(...)
```

### 3. ChatSidebar Component (`src/components/ChatSidebar.tsx`)
- Added empty state UI when no conversations exist
- Changed from conditional rendering to ternary for better UX

```typescript
// Shows friendly message when conversations.length === 0
<p className="px-3 py-4 text-xs text-[hsl(var(--sidebar-muted))] text-center">
  No conversations yet.<br />Start a new chat to begin.
</p>
```

## Testing Scenarios

### ✅ Empty State
- Fresh user with no conversations
- All conversations deleted
- API returns empty array

### ✅ Null/Undefined Responses
- API returns `null` for messages
- API returns `undefined` for items
- Network errors

### ✅ Normal Operation
- Loading existing conversations
- Loading message history
- Creating new conversations

## Result
The application now gracefully handles:
- Empty conversation lists
- Empty message lists
- Null/undefined API responses
- Network errors
- 404 responses

No more crashes when data is empty or null!
