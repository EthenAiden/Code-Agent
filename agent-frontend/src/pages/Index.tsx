import { useState, useRef, useEffect, useCallback } from "react";
import { flushSync } from "react-dom";
import { ArrowRight, ArrowLeft } from "lucide-react";
import ChatMessage, { type Message } from "@/components/ChatMessage";
import ChatInput from "@/components/ChatInput";
import { apiClient } from "@/lib/api";
import { toast } from "sonner";

interface Project {
  id: string;
  name: string;
  icon: string;
  editedTime: string;
  thumbnail: string;
  badge?: string;
  description?: string;
}

export default function Index() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isTyping, setIsTyping] = useState(false);
  const [activeProjectId, setActiveProjectId] = useState<string | null>(null);
  const [activeProjectName, setActiveProjectName] = useState<string>("");
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeTab, setActiveTab] = useState<"projects" | "recent" | "templates">("projects");
  const skipNextLoadRef = useRef(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Load projects on mount
  useEffect(() => {
    loadProjects();
  }, []);

  const loadProjects = async () => {
    try {
      const response = await apiClient.listSessions(50, 0);
      const loadedProjects: Project[] = (response.items || []).map((item: any) => ({
        id: item.conversation_id || item.project_id,
        name: item.first_message || item.name || "未命名项目",
        icon: item.icon || "💬",
        editedTime: formatTime(item.updated_at || item.last_message_timestamp),
        thumbnail: item.thumbnail || "bg-gradient-to-br from-gray-100 to-gray-200",
        description: item.description,
      }));
      setProjects(loadedProjects);
    } catch (error) {
      console.error("Failed to load projects:", error);
    }
  };

  const formatTime = (timestamp: string) => {
    if (!timestamp) return "刚刚";
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const days = Math.floor(diff / (1000 * 60 * 60 * 24));
    
    if (hours < 1) return "刚刚";
    if (hours < 24) return `${hours} 小时前`;
    if (days < 30) return `${days} 天前`;
    return date.toLocaleDateString();
  };

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, isTyping]);

  const handleSend = useCallback(async (text: string) => {
    let projectId = activeProjectId;
    let projectName = text.slice(0, 50); // Use first 50 chars as project name

    // Create new project if needed
    if (!projectId) {
      try {
        const response = await apiClient.createSession();
        projectId = response.conversation_id;
        skipNextLoadRef.current = true;
        setActiveProjectId(projectId);
        setActiveProjectName(projectName);
        
        // Add to projects list
        const newProject: Project = {
          id: projectId,
          name: projectName,
          icon: "💬",
          editedTime: "刚刚",
          thumbnail: "bg-gradient-to-br from-blue-100 to-purple-100",
        };
        setProjects((prev) => [newProject, ...prev]);
      } catch (error) {
        toast.error("创建项目失败：" + (error as Error).message);
        return;
      }
    }

    // Add user message to UI
    const userMsg: Message = {
      id: `${projectId}-user-${Date.now()}`,
      role: "user",
      content: text,
    };
    setMessages((prev) => [...prev, userMsg]);
    setIsTyping(true);

    // Create assistant message placeholder
    const assistantMsgId = `${projectId}-assistant-${Date.now()}`;
    const assistantMsg: Message = {
      id: assistantMsgId,
      role: "assistant",
      content: "",
    };
    setMessages((prev) => [...prev, assistantMsg]);

    // Send message to API with streaming
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
  }, [activeProjectId]);

  const handleBackToHome = () => {
    setActiveProjectId(null);
    setActiveProjectName("");
    setMessages([]);
    loadProjects(); // Reload projects to show the new one
  };

  const handleSelectProject = (project: Project) => {
    setActiveProjectId(project.id);
    setActiveProjectName(project.name);
    loadProjectMessages(project.id);
  };

  const loadProjectMessages = async (projectId: string) => {
    try {
      const apiMessages = await apiClient.getMessages(projectId);
      const msgs: Message[] = (apiMessages || []).map((msg) => ({
        id: `${msg.conversation_id}-${msg.message_index}`,
        role: msg.role,
        content: msg.content,
      }));
      setMessages(msgs);
    } catch (error) {
      console.error("Failed to load messages:", error);
      setMessages([]);
    }
  };

  const hasMessages = messages.length > 0;
  const isInProject = activeProjectId !== null;

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background">
      <main className="relative flex flex-1 flex-col overflow-hidden">
        {isInProject ? (
          <>
            {/* Back button header */}
            <header className="flex h-14 items-center px-4 border-b border-[hsl(var(--border))]">
              <button
                onClick={handleBackToHome}
                className="flex items-center gap-2 text-sm font-medium text-foreground/60 hover:text-foreground transition-colors"
              >
                <ArrowLeft className="h-4 w-4" />
                返回主页
              </button>
              {activeProjectName && (
                <span className="ml-4 text-sm font-semibold text-foreground">
                  {activeProjectName}
                </span>
              )}
            </header>

            {/* Chat messages */}
            <div ref={scrollRef} className="flex-1 overflow-y-auto">
              <div className="mx-auto max-w-[680px] px-4 py-8 space-y-4">
                {messages.map((msg) => (
                  <ChatMessage key={msg.id} message={msg} />
                ))}
                {isTyping && (
                  <div className="flex items-start gap-3">
                    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-[hsl(var(--border))]">
                      <div className="flex gap-0.5">
                        <span className="h-1 w-1 rounded-full bg-foreground/40 animate-bounce [animation-delay:0ms]" />
                        <span className="h-1 w-1 rounded-full bg-foreground/40 animate-bounce [animation-delay:150ms]" />
                        <span className="h-1 w-1 rounded-full bg-foreground/40 animate-bounce [animation-delay:300ms]" />
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>
            
            {/* Input at bottom */}
            <div className="border-t border-[hsl(var(--border))]">
              <div className="mx-auto max-w-[680px] px-4 py-4">
                <ChatInput onSend={handleSend} disabled={isTyping} />
              </div>
            </div>
          </>
        ) : (
          <>
            {/* Welcome screen with centered input */}
            <div ref={scrollRef} className="flex-1 overflow-y-auto">
              <div className="flex flex-col min-h-full">
                {/* Hero section with centered input - moved down */}
                <div className="flex-1 flex flex-col items-center justify-center px-4 pt-20 pb-20">
                  <div className="w-full max-w-[680px] text-center">
                    
                    
                    {/* Centered input */}
                    <div className="w-full">
                      <ChatInput onSend={handleSend} disabled={isTyping} />
                    </div>
                  </div>
                </div>

                {/* Projects section with rounded container */}
                <div className="w-full px-8 pb-12">
                  <div className="max-w-[1200px] mx-auto">
                    {/* Container with rounded border */}
                    <div className="rounded-2xl border border-[hsl(var(--border))] bg-muted/30 p-8">
                      {/* Tabs and Browse all */}
                      <div className="flex items-center justify-between mb-8">
                      <div className="flex gap-6">
                        <button
                          onClick={() => setActiveTab("projects")}
                          className={`text-sm font-medium pb-2 border-b-2 transition-colors ${
                            activeTab === "projects"
                              ? "border-foreground text-foreground"
                              : "border-transparent text-foreground/60 hover:text-foreground"
                          }`}
                        >
                          My projects
                        </button>
                        <button
                          onClick={() => setActiveTab("recent")}
                          className={`text-sm font-medium pb-2 border-b-2 transition-colors ${
                            activeTab === "recent"
                              ? "border-foreground text-foreground"
                              : "border-transparent text-foreground/60 hover:text-foreground"
                          }`}
                        >
                          Recently viewed
                        </button>
                        <button
                          onClick={() => setActiveTab("templates")}
                          className={`text-sm font-medium pb-2 border-b-2 transition-colors ${
                            activeTab === "templates"
                              ? "border-foreground text-foreground"
                              : "border-transparent text-foreground/60 hover:text-foreground"
                          }`}
                        >
                          Templates
                        </button>
                      </div>
                      <button className="flex items-center gap-2 text-sm font-medium text-foreground/60 hover:text-foreground transition-colors">
                        Browse all
                        <ArrowRight className="h-4 w-4" />
                      </button>
                    </div>

                    {/* Project cards grid */}
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                      {projects.map((project) => (
                        <div
                          key={project.id}
                          className="group cursor-pointer"
                          onClick={() => handleSelectProject(project)}
                        >
                          {/* Thumbnail */}
                          <div className={`relative aspect-[16/10] rounded-xl overflow-hidden mb-3 ${project.thumbnail} shadow-lg group-hover:shadow-xl transition-shadow`}>
                            {project.badge && (
                              <div className="absolute top-3 left-3 px-2 py-1 bg-background/90 backdrop-blur-sm rounded text-xs font-medium">
                                {project.badge}
                              </div>
                            )}
                          </div>
                          
                          {/* Project info */}
                          <div className="flex items-start gap-3">
                            <div className="flex-shrink-0 w-8 h-8 rounded-lg bg-gradient-to-br from-yellow-400 to-orange-500 flex items-center justify-center text-lg shadow-sm">
                              {project.icon}
                            </div>
                            <div className="flex-1 min-w-0">
                              <h3 className="text-sm font-semibold text-foreground group-hover:text-foreground/80 transition-colors truncate">
                                {project.name}
                              </h3>
                              <p className="text-xs text-foreground/50 mt-0.5">
                                {project.editedTime}
                              </p>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </>
        )}
      </main>
    </div>
  );
}
