#!/bin/bash

# Ensure yq is installed
if ! command -v yq &> /dev/null
then
    echo "yq could not be found. Please install yq to proceed."
    exit
fi

# File path
CONFIG_FILE="./deploy-batch.yaml"
DEPLOY_SCRIPT="./deploy-dhis2.sh"

# Display and select a set for deployment
select_set() {
    echo "Available Sets:"
    set_names=$(yq e '.sets[].name' "$CONFIG_FILE")
    IFS=$'\n' read -rd '' -a set_array <<<"$set_names"

    for i in "${!set_array[@]}"; do
        echo "$((i+1)). ${set_array[i]}"
    done

    echo -n "Enter the number of the set you want to select, or 'c' to cancel: "
    read selection

    if [ "$selection" == "c" ]; then
        echo "Selection cancelled."
        exit
    fi

    if ! [[ "$selection" =~ ^[0-9]+$ ]] || [ "$selection" -le 0 ] || [ "$selection" -gt "${#set_array[@]}" ]; then
        echo "Invalid selection."
        exit
    fi

    selected_set=$(yq e ".sets[$((selection-1))]" "$CONFIG_FILE")
    if [ -z "$selected_set" ]; then
        echo "Set not found."
    # else
    #     echo "----------------------------------------"
    #     echo "Selected Set:"
    #     echo "$selected_set"
    #     echo "----------------------------------------"
    fi

    # store the IMAGE_REPOSITORY of the selected set
    export IMAGE_REPOSITORY=$(echo "$selected_set" | yq e '.IMAGE_REPOSITORY' -)


    # Set environment variables based on preferences and selected set
    # echo "Setting environment variables based on preferences and selected set..."
    preferences=$(yq e '.preferences' "$CONFIG_FILE")
    # echo "prefs $preferences"
    for pref in $(echo "$preferences" | yq e 'keys | .[]' -); do
        export ${pref}=$(echo "$preferences" | yq e ".$pref" -)
    done

    # check if the IM_PREFIX_MULTI is set
    if [ -z "$IM_PREFIX_MULTI" ]; then
        echo -c "Instance prefix is not set. Please provide a prefix for the instances: "
        read im_prefix
        export IM_PREFIX_MULTI=$im_prefix
    fi


# Now, loop over the instances and set the environment variables, then call deploy-dhis2.sh script for each instance
    instances=$(echo "$selected_set" | yq e '.instances' -)

    # Loop over the instances first just to list the instances that will be created
    # and get confirmation to continue
    echo "----------------------------------------"
    echo "Instances to be created:"
    for instance in $(echo "$instances" | yq e 'keys | .[]' -); do
        instance_data=$(echo "$instances" | yq e ".$instance" -)
        # add 1 to the index to start from 1
        echo "Instance $((instance+1)):"
        for key in $(echo "$instance_data" | yq e 'keys | .[]' -); do
            if [ "$key" == "name" ]; then
                # echo with tabs and newline
                echo -e "\tName: ${IM_PREFIX_MULTI}$(echo "$instance_data" | yq e ".$key" -)"
            elif [ "$key" == "description" ]; then
                echo -e "\tDescription: $(echo "$instance_data" | yq e ".$key" -)"
            fi
        done
    done
    echo "----------------------------------------"

    echo -n "Do you want to continue? (y/n): "
    read confirmation
    if [ "$confirmation" != "y" ]; then
        echo "Operation cancelled."
        exit
    fi

    for instance in $(echo "$instances" | yq e 'keys | .[]' -); do
        instance_data=$(echo "$instances" | yq e ".$instance" -)
        for key in $(echo "$instance_data" | yq e 'keys | .[]' -); do
            export "$key"="$(echo "$instance_data" | yq e ".$key" -)"
        done

        echo "Deploying DHIS2 instance $((instance+1))..."
        ${DEPLOY_SCRIPT} $IM_GROUP_MULTI ${IM_PREFIX_MULTI}${name} ${description}   #  deploy_dhis2

    done


}

# Main script execution
select_set