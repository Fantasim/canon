// Runs the conformance entry point alone and prints its failure count (CPP-06).
#include <cstdio>

#include "pipeline.gen.h"

int main() {
    std::printf("failures: %d\n", sov::gen::conformance::RunPipelineConformance());
    return 0;
}
