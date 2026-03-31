# API Integration Guide

## Overview
The frontend has been integrated with the backend API. All mock data has been removed and replaced with real API calls.

## Environment Setup

### Backend (agent-server)
1. Ensure the backend is running on port 8888
2. Database should be initialized and running
3. Check `.env` file for correct configuration

```bash
cd agent-server
go run main.go
```

### Frontend (agent-frontend)
1. Install dependencies:
```bash
cd agent-frontend
npm install
```

2. Create `.env` file (already created):
```env
VITE_API_BASE_URL=http://localhost:8888
VITE_USER_ID=550e8400-e29b-41d4-a716-446655440000
```

3. Start the development server:
```bash
npm run dev
```

The frontend will run on `http://localhost:5173`

## API Integration Details

### API Client (`src/lib/api.ts`)
- Centralized API client with all backend endpoints
- Automatic header injection (X-User-ID)
- Error handling and type safety

### Endpoints Used
1. **Session Management**
   - `POST /api/v1/sessions` - Create new session
   - `GET /api/v1/sessions` - List all sessions
   - `GET /api/v1/sessions/:id` - Get session details
   - `DELETE /api/v1/sessions/:id` - Delete session

2. **Chat**
   - `POST /api/v1/conversation/:id/chat` - Send message
   - `GET /api/v1/conversation/:id/messages` - Get message history

### Changes Made
1. Removed all mock data (MOCK_CONVERSATIONS, MOCK_REPLIES)
2. Added API client with TypeScript types
3. Updated Index.tsx to use real API calls:
   - Load sessions on mount
   - Load messages when selecting conversation
   - Create session on first message
   - Send messages to backend
   - Delete conversations
4. Added error handling with toast notifications
5. Added loading states

### Features
- ✅ Create new conversations
- ✅ List all conversations
- ✅ Load conversation history
- ✅ Send and receive messages
- ✅ Delete conversations
- ✅ Error handling with user feedback
- ✅ Loading states

## Testing
1. Start backend server
2. Start frontend dev server
3. Open browser to `http://localhost:5173`
4. Test creating conversations, sending messages, and deleting conversations

## Notes
- User ID is hardcoded in `.env` for development
- Backend must be running before frontend
- CORS is configured to allow localhost:5173
