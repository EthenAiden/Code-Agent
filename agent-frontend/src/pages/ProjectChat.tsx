import { useState, useRef, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { flushSync } from "react-dom";
import { ArrowLeft, Sparkles } from "lucide-react";
import ChatMessage, { type Message } from "@/components/ChatMessage";
import ChatInput from "@/components/ChatInput";
import { apiClient } from "@/lib/api";
import { toast } from "sonner";

export default function ProjectChat() {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();
  const [messages, setMessages] = useState<Message[]>([]);
  const [isTyping, setIsTyping] = useState(false);
  const [projectName, setProjectName] = useState<string>("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (projectId) {
      loadProjectMessages(projectId);
    }
  }, [projectId]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, isTyping]);

  const loadProjectMessages = async (id: string) => {
    try {
      const apiMessages = await apiClient.getMessages(id);
      const msgs: Message[] = (apiMessages || []).map((msg) => ({
        id: `${msg.conversation_id}-${msg.message_index}`,
        role: msg.role,
        content: msg.content,
      }));
      setMessages(msgs);
      
      if (msgs.length > 0) {
        const firstUserMsg = msgs.find(m => m.role === 'user');
        if (firstUserMsg) {
          setProjectName(firstUserMsg.content.slice(0, 50));
        }
      }
    } catch (error) {
      console.error("Failed to load messages:", error);
      toast.error("加载消息失败");
      setMessages([]);
    }
  };

  const handleSend = async (text: string) => {
    if (!projectId) return;

    const userMsg: Message = {
      id: `${projectId}-user-${Date.now()}`,
      role: "user",
      content: text,
    };
    setMessages((prev) => [...prev, userMsg]);
    setIsTyping(true);

    const assistantMsgId = `${projectId}-assistant-${Date.now()}`;
    const assistantMsg: Message = {
      id: assistantMsgId,
      role: "assistant",
      content: "",
    };
    setMessages((prev) => [...prev, assistantMsg]);

    try {
      await apiClient.sendMessage(projectId, text, (chunk) => {
        flushSync(() => {
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === assistantMsgId
                ? { ...msg, content: msg.content + chunk }
                : msg
            )
          );
        });
      });
    } catch (error) {
      toast.error("发送消息失败：" + (error as Error).message);
      setMessages((prev) => prev.filter((msg) => msg.id !== assistantMsgId));
    } finally {
      setIsTyping(false);
    }
  };

  const handleBackToHome = () => {
    navigate("/");
  };

  return (
    <div className="flex h-screen w-full overflow-hidden bg-gradient-to-br from-background via-background to-muted/10">
      <main className="relative flex flex-1 flex-col overflow-hidden">
        {/* Enhanced header */}
        <header className="relative flex h-16 items-center px-6 border-b border-border/50 bg-card/50 backdrop-blur-md">
          <div className="absolute inset-0 bg-gradient-to-r from-blue-500/5 to-purple-500/5 pointer-events-none"></div>
          <div className="relative flex items-center gap-4 w-full">
            <button
              onClick={handleBackToHome}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium text-foreground/60 hover:text-foreground hover:bg-muted/50 transition-all"
            >
              <ArrowLeft className="h-4 w-4" />
              返回主页
            </button>
            {projectName && (
              <>
                <div className="h-4 w-px bg-border/50"></div>
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-gradient-to-br from-blue-500 to-purple-500">
                    <Sparkles className="h-3.5 w-3.5 text-white" />
                  </div>
                  <span className="text-sm font-semibold text-foreground truncate max-w-md">
                    {projectName}
                  </span>
                </div>
              </>
            )}
          </div>
        </header>

        {/* Chat messages with enhanced styling */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[720px] px-4 py-8 space-y-6">
            {messages.map((msg, index) => (
              <div
                key={msg.id}
                className="animate-in fade-in slide-in-from-bottom-2"
                style={{ animationDelay: `${index * 50}ms`, animationFillMode: 'backwards' }}
              >
                <ChatMessage message={msg} />
              </div>
            ))}
            {isTyping && (
              <div className="flex items-start gap-3 animate-in fade-in slide-in-from-bottom-2">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl border border-border/50 bg-muted/50 backdrop-blur-sm">
                  <div className="flex gap-1">
                    <span className="h-1.5 w-1.5 rounded-full bg-foreground/40 animate-bounce [animation-delay:0ms]" />
                    <span className="h-1.5 w-1.5 rounded-full bg-foreground/40 animate-bounce [animation-delay:150ms]" />
                    <span className="h-1.5 w-1.5 rounded-full bg-foreground/40 animate-bounce [animation-delay:300ms]" />
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Enhanced input area */}
        <div className="relative border-t border-border/50 bg-card/30 backdrop-blur-md">
          <div className="absolute inset-0 bg-gradient-to-t from-muted/10 to-transparent pointer-events-none"></div>
          <div className="relative mx-auto max-w-[720px] px-4 py-4">
            <ChatInput onSend={handleSend} disabled={isTyping} />
          </div>
        </div>
      </main>
    </div>
  );
}
