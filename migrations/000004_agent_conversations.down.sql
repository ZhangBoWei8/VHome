-- Reverses 000004. Dropping these tables deletes every stored conversation, so
-- only use it on a throwaway database. agent_messages goes first because it
-- holds the foreign key.
DROP TABLE IF EXISTS agent_messages;
DROP TABLE IF EXISTS agent_conversations;
