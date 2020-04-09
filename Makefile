BUILD_DIR=./build
OUT_DIR=./out

clean_build_dir:
	rm -rf $(BUILD_DIR)


create_build_dir: clean_build_dir
	mkdir -p $(BUILD_DIR)

clean_out_dir:
	rm -rf $(OUT_DIR)

create_out_dir: clean_out_dir
	mkdir -p $(OUT_DIR)

compile: create_build_dir create_out_dir
	./build_distributions.sh
	rm -rf $(BUILD_DIR)
