// TestReaderMatchesV020's subject: the reader of this compiler's canon_runtime_json.h, as written.
#include <string>
#include <string_view>

#include <nlohmann/json.hpp>

#include "canon_runtime_json.h"

bool SubjectText(std::string_view text, nlohmann::json& doc, std::string& error, canon::json::Float32Tokens& tokens,
                 canon::json::KeyOrders* orders) {
    return canon::json::ParseTextFile("T", text, doc, error, tokens, orders);
}

bool SubjectData(const std::string& text, std::string_view schema, nlohmann::json& doc, std::string& error,
                 canon::json::Float32Tokens& tokens, canon::json::KeyOrders* orders) {
    return canon::json::ParseDataFile("D", text, schema, doc, error, tokens, orders);
}
