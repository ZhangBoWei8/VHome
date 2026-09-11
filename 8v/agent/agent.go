package agent

import (
	"context"
	"fmt"
	"strings"

	"vhome/8v/llm"
	"vhome/8v/tools"
	"vhome/internal/service"
)

// maxToolRounds bounds one answer: the model may call tools this many times
// before it has to reply with words.
const maxToolRounds = 5

// Agent answers one turn at a time. It holds no conversation state: the thread
// lives in MySQL and is loaded per turn, which is what makes a single instance
// safe to share across concurrent requests.
type Agent struct {
	client        llm.Client
	tools         *tools.Registry
	conversations *service.AgentConversationService

	// allowWrites gates every tool marked Dangerous. Set it to false to run
	// the assistant in a read-only mode.
	allowWrites bool
}

// Sink receives a turn's progress as it happens. It is the seam that lets the
// same loop serve a buffered JSON response and a streamed one; every method
// may be called from the request goroutine only.
type Sink interface {
	// OnConversation fires once, as soon as the thread is known. A new thread
	// gets its id here, before any model call.
	OnConversation(conversationID uint64) error

	// OnToolCall fires before each tool runs, so the client can show what the
	// agent is doing during a long turn.
	OnToolCall(name string) error

	// OnToken fires for each fragment of the final answer.
	OnToken(text string) error
}

// Result is what one completed turn produced.
type Result struct {
	ConversationID uint64
	Answer         string
}

func New(client llm.Client, registry *tools.Registry, conversations *service.AgentConversationService) *Agent {
	return &Agent{
		client:        client,
		tools:         registry,
		conversations: conversations,
		allowWrites:   true,
	}
}

const baseSystemPrompt = `你是 8V，一个家庭助手，负责家庭健康、家务、库存管理、提醒和家庭事务。

## 工作方式
- 需要家庭的真实数据时必须调用工具，绝不凭记忆或猜测回答库存、账单、饮食和提醒的具体数字。
- 工具返回的 JSON 是事实来源。基于事实做判断和建议，但不要编造事实里没有的内容。
- 查不到数据时如实说明，并说清你查了什么，不要用推测填补。
- 回答用简体中文，简洁自然，像家人之间说话。不要罗列 JSON，也不要提工具名。

## 写操作
- pantry_add、pantry_discard、expense_record、memo_create 会真正修改家庭数据。
- 只有当用户明确表达了这个意图时才调用，不要主动替用户记账、入库或设提醒。
- 信息不全时先问清楚再写，不要用默认值蒙混过去。
- 写入成功后，用一句话复述你实际写了什么（金额、日期、数量、提醒时间），方便用户核对。

## 修改和删除
- 修改和删除库存或账单需要先查到这条记录的 id 和 version，把查询结果里的这两个值原样传给写工具，不要自己编。
- 如果工具提示记录已被他人修改，重新查询后再试一次。

## 上下文
- 你能看到当前对话之前的消息，用户的追问（“那第二个呢”“把刚才那条改成 8 点”）指的就是上文提到的内容。
- 但上文里的库存数量、账单金额这类数据可能已经过时，涉及具体数字时重新查一次，不要直接复述历史消息里的数字。

## 记忆
- 你记住的事情会写在系统提示的“你记住的事情”里，每轮自动带上，不用再查。
- 只有长期有效的偏好、习惯和限制才值得记：口味、忌口、过敏、固定安排、家里的长期约束。
- 会变化的数据一律不要记：库存数量、账单金额、提醒时间、谁在家。这些必须每次调用工具查，记下来就会过期出错。
- 用户明确要求“记住”，或说出了一条明显长期有效的偏好时，才调用 memory_remember。不要把每句闲聊都存进去。
- 判断作用域：全家都适用的用 HOUSEHOLD，只和当前成员有关的私人偏好用 MEMBER。
- 记住之后用一句话说明你记下了什么；如果这条已经记过，就直接说你已经知道了，不要假装是新记的。
- 用户要求忘记或更正时，先用 memory_list 拿到 id 和 version，再调用 memory_forget。`

func (a *Agent) systemPrompt(snapshot tools.HouseholdSnapshot) string {
	var b strings.Builder

	b.WriteString(baseSystemPrompt)
	b.WriteString("\n\n")
	b.WriteString(snapshot.PromptSection())

	return b.String()
}

// Run answers one message. A zero conversationID opens a new thread.
//
// Everything the turn produces is persisted together at the end: writing the
// user message eagerly would leave an orphaned question in the thread whenever
// the provider call fails.
func (a *Agent) Run(
	ctx context.Context,
	actor service.AuthenticatedIdentity,
	conversationID uint64,
	input string,
	sink Sink,
) (Result, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Result{ConversationID: conversationID}, nil
	}

	// An existing thread is verified and loaded up front, so an id belonging
	// to somebody else is rejected before the model is ever called.
	//
	// A new thread is NOT created here: it is opened when the turn is
	// persisted. Creating it first would leave an empty conversation in the
	// member's sidebar every time a turn failed.
	var history []llm.Message

	if conversationID != 0 {
		conversation, err := a.conversations.Get(ctx, actor, conversationID)
		if err != nil {
			return Result{}, err
		}

		if sink != nil {
			if err := sink.OnConversation(conversation.ID); err != nil {
				return Result{}, err
			}
		}

		history, err = a.loadHistory(ctx, actor, conversation.ID)
		if err != nil {
			return Result{}, err
		}
	}

	// The household facts are refreshed on every run: the clock has moved and
	// someone may have come home since the previous message.
	snapshot := a.tools.Snapshot(ctx, actor)

	messages := make([]llm.Message, 0, len(history)+4)
	messages = append(messages, llm.System(a.systemPrompt(snapshot)))
	messages = append(messages, history...)
	messages = append(messages, llm.User(input))

	// turn is what gets persisted: the user's question and the assistant's
	// spoken answer, nothing else.
	//
	// Tool traffic is deliberately NOT stored. Tool results describe state that
	// changes — stock counts, balances, who is home — so a stored copy is stale
	// the moment it is written, and the user never sees it anyway. A follow-up
	// that needs a detail re-runs the tool and gets fresh data instead.
	//
	// The tool_calls/tool rows must be dropped as a *group*: an assistant
	// message carrying tool_calls is rejected by the provider if its matching
	// tool results are missing, so keeping one without the other would be worse
	// than keeping neither.
	turn := []llm.Message{llm.User(input)}

	invocation := tools.Invocation{
		Actor:       actor,
		AllowWrites: a.allowWrites,
	}

	answer := ""
	for round := 0; round < maxToolRounds; round++ {
		resp, chatErr := a.chat(ctx, messages, sink)
		if chatErr != nil {
			return Result{}, chatErr
		}

		if len(resp.ToolCalls) == 0 {
			answer = strings.TrimSpace(resp.Content)
			messages = append(messages, llm.Assistant(answer))
			turn = append(turn, llm.Assistant(answer))

			break
		}

		// Added to the live context for this turn only — never to `turn`.
		messages = append(messages, llm.AssistantWithTools(resp.Content, resp.ToolCalls))

		for _, call := range resp.ToolCalls {
			if sink != nil {
				if err := sink.OnToolCall(call.Function.Name); err != nil {
					return Result{}, err
				}
			}

			result, execErr := a.tools.Execute(
				ctx,
				invocation,
				call.Function.Name,
				string(call.Function.Arguments),
			)
			if execErr != nil {
				// Registry.Execute only returns messages that are safe for the
				// model to read and, if it chooses, to relay to the user.
				result = "调用失败：" + execErr.Error()
			}

			messages = append(messages, llm.ToolResult(call.ID, call.Function.Name, result))
		}
	}

	if answer == "" {
		answer = "我查询的步骤太多了，没能整理出结果。可以把问题拆得更具体一些再问我吗？"

		// The loop ran out of rounds without a spoken answer. Persist the
		// fallback so the stored thread matches what the user was shown.
		turn = append(turn, llm.Assistant(answer))

		if sink != nil {
			if err := sink.OnToken(answer); err != nil {
				return Result{}, err
			}
		}
	}

	storedID, err := a.persist(ctx, actor, conversationID, turn, input, sink)
	if err != nil {
		return Result{}, err
	}

	return Result{ConversationID: storedID, Answer: answer}, nil
}

// chat asks the model for one response, streaming the answer fragments to the
// sink when the client supports it.
func (a *Agent) chat(ctx context.Context, messages []llm.Message, sink Sink) (llm.ChatResponse, error) {
	definitions := a.tools.Definitions()

	streamer, ok := a.client.(llm.StreamingClient)
	if !ok || sink == nil {
		return a.client.Chat(ctx, messages, definitions)
	}

	return streamer.ChatStream(ctx, messages, definitions, func(delta llm.Delta) error {
		if delta.Content == "" {
			return nil
		}

		return sink.OnToken(delta.Content)
	})
}

func (a *Agent) loadHistory(ctx context.Context, actor service.AuthenticatedIdentity, conversationID uint64) ([]llm.Message, error) {
	stored, err := a.conversations.ContextMessages(ctx, actor, conversationID)
	if err != nil {
		return nil, err
	}

	replayed, err := replayMessages(stored)
	if err != nil {
		return nil, err
	}

	return trimContext(replayed), nil
}

// persist stores the turn, opening the thread first when this was the first
// message of a new conversation. It returns the id the turn was stored under.
func (a *Agent) persist(
	ctx context.Context,
	actor service.AuthenticatedIdentity,
	conversationID uint64,
	turn []llm.Message,
	input string,
	sink Sink,
) (uint64, error) {
	stored, err := turnMessages(turn)
	if err != nil {
		return 0, err
	}

	if conversationID == 0 {
		conversation, createErr := a.conversations.Create(ctx, actor)
		if createErr != nil {
			return 0, createErr
		}

		conversationID = conversation.ID

		if sink != nil {
			if err := sink.OnConversation(conversationID); err != nil {
				return 0, err
			}
		}
	}

	if err := a.conversations.AppendTurn(ctx, actor, conversationID, stored, input); err != nil {
		return 0, fmt.Errorf("persist agent turn: %w", err)
	}

	return conversationID, nil
}
