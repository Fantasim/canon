// TestReaderMatchesV020's oracle: v0.2.0's reader (testdata/reader/v0.2.0), in an inline namespace of its own.
#include <string>
#include <string_view>

#include <nlohmann/json.hpp>

#define rt_v1 rt_v1_v020
#include "reader/v0.2.0/canon_runtime_json.h"
#undef rt_v1

bool OracleText(std::string_view text, nlohmann::json& doc, std::string& error, canon::json::Float32Tokens& tokens,
                canon::json::KeyOrders* orders) {
    return canon::json::ParseTextFile("T", text, doc, error, tokens, orders);
}

bool OracleData(const std::string& text, std::string_view schema, nlohmann::json& doc, std::string& error,
                canon::json::Float32Tokens& tokens, canon::json::KeyOrders* orders) {
    return canon::json::ParseDataFile("D", text, schema, doc, error, tokens, orders);
}
