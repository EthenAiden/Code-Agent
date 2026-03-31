import ChatInput from "@/components/ChatInput";

interface WelcomeScreenProps {
  onSend: (text: string) => void;
}

export default function WelcomeScreen({ onSend }: WelcomeScreenProps) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center px-4">
      <h1 className="mb-8 text-[28px] font-semibold text-foreground">
        我能帮你什么？
      </h1>
      <ChatInput onSend={onSend} centered />
    </div>
  );
}
